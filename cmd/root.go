// Package cmd implements the cobra CLI interface for upkeep.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/engine"
	"github.com/teknikqa/upkeep/internal/logging"
	"github.com/teknikqa/upkeep/internal/provider"
	"github.com/teknikqa/upkeep/internal/state"
)

var (
	cfgFile          string
	dryRun           bool
	yes              bool
	verbose          bool
	list             bool
	retryFailed      bool
	runDeferred      bool
	forceInteractive bool
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "upkeep [provider...]",
	Short: "A Go-based system updater for macOS",
	Long: `upkeep is a shell-independent system updater for macOS that manages
updates for Homebrew, npm, pip, Rust, VS Code extensions, and more.

Examples:
  upkeep                    # Update all available providers
  upkeep brew npm           # Update only brew and npm
  upkeep --dry-run          # Scan and show what would be updated
  upkeep --yes              # Update without confirmation prompt
  upkeep --retry-failed     # Re-run providers that failed last time
  upkeep --run-deferred     # Run deferred auth-required cask updates
  upkeep --list             # List all available providers`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if list {
			return runList()
		}
		return runUpdate(cmd, args)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default: ~/.config/upkeep/config.yaml)")
	rootCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Scan and show summary without performing any updates")
	rootCmd.PersistentFlags().BoolVarP(&yes, "yes", "y", false, "Skip confirmation prompt and update immediately")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Show full provider output on console (in addition to log file)")
	rootCmd.PersistentFlags().BoolVarP(&list, "list", "l", false, "List all available providers and exit")
	rootCmd.PersistentFlags().BoolVar(&retryFailed, "retry-failed", false, "Re-run only providers that failed in the last run")
	rootCmd.PersistentFlags().BoolVar(&runDeferred, "run-deferred", false, "Execute deferred auth-required cask updates")
	rootCmd.PersistentFlags().BoolVar(&forceInteractive, "force-interactive", false, "Force interactive mode for auth-required casks (overrides defer strategy)")
}

// runList prints all registered providers.
func runList() error {
	names := provider.List()
	if len(names) == 0 {
		fmt.Println("No providers registered")
		return nil
	}
	fmt.Println("Available providers:")
	for _, name := range names {
		p, _ := provider.Get(name)
		fmt.Printf("  %-20s %s\n", name, p.DisplayName())
	}
	return nil
}

// runUpdate is the main update pipeline entry point.
func runUpdate(cmd *cobra.Command, args []string) error {
	// Load config.
	cfg, err := config.Load(cfgFile)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	// Override auth_strategy if --force-interactive is set.
	if forceInteractive {
		cfg.Providers.BrewCask.AuthStrategy = "force-interactive"
	}

	// Set up logger.
	logLevel := logging.ParseLevel(cfg.Logging.Level)
	logger := logging.New(cfg.Logging.Dir, logLevel)
	defer logger.Close()

	// Load state.
	st, err := state.Load("")
	if err != nil {
		// Non-fatal: log and continue with empty state.
		logger.Warn("loading state: %v", err)
		st = state.New("")
	}

	// Collect all registered providers, filtered by enabled config.
	allProviders := provider.GetAll()
	enabledProviders := filterEnabledProviders(allProviders, cfg)

	// Set up engine.
	eng := engine.New(cfg, st, logger)

	opts := engine.Options{
		Providers:        enabledProviders,
		ProviderNames:    args,
		DryRun:           dryRun,
		Yes:              yes,
		Verbose:          verbose,
		RetryFailed:      retryFailed,
		RunDeferred:      runDeferred,
		ForceInteractive: forceInteractive,
		Output:           os.Stdout,
	}

	return eng.Run(cmd.Context(), opts)
}

// filterEnabledProviders returns providers that are enabled in the config.
func filterEnabledProviders(providers []provider.Provider, cfg *config.Config) []provider.Provider {
	enabled := map[string]bool{
		"brew":      cfg.Providers.Brew.Enabled,
		"brew-cask": cfg.Providers.BrewCask.Enabled,
		"npm":       cfg.Providers.Npm.Enabled,
		"composer":  cfg.Providers.Composer.Enabled,
		"pip":       cfg.Providers.Pip.Enabled,
		"rust":      cfg.Providers.Rust.Enabled,
		"editor":    cfg.Providers.Editor.Enabled,
		"omz":       cfg.Providers.Omz.Enabled,
		"vim":       cfg.Providers.Vim.Enabled,
		"vagrant":   cfg.Providers.Vagrant.Enabled,
	}

	var result []provider.Provider
	for _, p := range providers {
		if v, ok := enabled[p.Name()]; !ok || v {
			result = append(result, p)
		}
	}
	return result
}
