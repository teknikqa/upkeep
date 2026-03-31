package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/ui"
)

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View and manage upkeep configuration",
	Long: `View and manage the upkeep configuration file.

Subcommands:
  show     Print the current effective configuration
  path     Print the configuration file path
  reset    Reset configuration to defaults
  edit     Interactively edit configuration (TUI)`,
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Print the current effective configuration as YAML",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}
		data, err := yaml.Marshal(cfg)
		if err != nil {
			return fmt.Errorf("marshalling config: %w", err)
		}
		fmt.Print(string(data))
		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the configuration file path",
	Run: func(cmd *cobra.Command, args []string) {
		path := cfgFile
		if path == "" {
			path = config.DefaultConfigPath()
		}
		fmt.Println(path)
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		if !ui.Confirm("Reset configuration to defaults? This will overwrite your current config.", false) {
			ui.PrintInfo("Reset cancelled.")
			return nil
		}

		path := cfgFile
		if path == "" {
			path = config.DefaultConfigPath()
		}

		if err := config.Save(config.Defaults(), path); err != nil {
			return fmt.Errorf("saving default config: %w", err)
		}
		ui.PrintInfo("Configuration reset to defaults at %s", path)
		return nil
	},
}

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Interactively edit configuration (TUI)",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		edited, saved, err := ui.RunConfigEditor(cfg)
		if err != nil {
			return fmt.Errorf("config editor: %w", err)
		}

		if !saved {
			ui.PrintInfo("No changes saved.")
			return nil
		}

		path := cfgFile
		if path == "" {
			path = config.DefaultConfigPath()
		}

		if err := config.Save(edited, path); err != nil {
			return fmt.Errorf("saving config: %w", err)
		}
		ui.PrintInfo("Configuration saved to %s", path)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configShowCmd, configPathCmd, configResetCmd, configEditCmd)
	rootCmd.AddCommand(configCmd)
}
