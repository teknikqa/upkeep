// Package engine orchestrates the scan → confirm → execute → report pipeline.
// It coordinates providers, manages state persistence, and drives the TUI output.
package engine

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
	"github.com/teknikqa/upkeep/internal/provider"
	"github.com/teknikqa/upkeep/internal/state"
	"github.com/teknikqa/upkeep/internal/ui"
)

// Options controls the engine pipeline.
type Options struct {
	// Providers is the list of providers to run (nil = all registered).
	Providers []provider.Provider
	// DryRun stops after the scan phase without executing updates.
	DryRun bool
	// Yes auto-confirms the update prompt.
	Yes bool
	// Verbose tees provider output to stdout in addition to the log file.
	Verbose bool
	// RetryFailed loads last-run state and runs only failed providers.
	RetryFailed bool
	// RunDeferred loads last-run state and executes deferred cask script.
	RunDeferred bool
	// ForceInteractive overrides auth_strategy to force-interactive for casks.
	ForceInteractive bool
	// ProviderNames filters to specific providers (from CLI positional args).
	ProviderNames []string
	// Output is where status lines are written (default: os.Stdout).
	Output io.Writer
}

// Engine orchestrates the full scan→confirm→execute→report pipeline.
type Engine struct {
	cfg    *config.Config
	state  *state.State
	logger *logging.Logger
}

// New creates an Engine with the provided config, state, and logger.
func New(cfg *config.Config, st *state.State, logger *logging.Logger) *Engine {
	return &Engine{cfg: cfg, state: st, logger: logger}
}

// Run executes the pipeline. Returns an error only for fatal failures.
func (e *Engine) Run(ctx context.Context, opts Options) error {
	out := opts.Output
	if out == nil {
		out = os.Stdout
	}

	// Handle --run-deferred.
	if opts.RunDeferred {
		return e.runDeferred(ctx, out, opts)
	}

	// Determine providers to run.
	providers, err := e.resolveProviders(opts)
	if err != nil {
		return err
	}

	// Handle --retry-failed.
	if opts.RetryFailed {
		failedNames := e.state.GetFailed()
		if len(failedNames) == 0 {
			fmt.Fprintln(out, "No failed providers from last run.")
			return nil
		}
		filtered := filterProvidersByName(providers, failedNames)
		if len(filtered) == 0 {
			fmt.Fprintln(out, "No registered providers match the failed names from last run.")
			return nil
		}
		providers = filtered
	}

	if len(providers) == 0 {
		fmt.Fprintln(out, "No providers to run.")
		return nil
	}

	// Build skip-lists from config.
	skipLists := buildSkipLists(e.cfg)

	// Phase 1: Scan.
	fmt.Fprintln(out, "Scanning for updates...")
	scanStart := time.Now()
	scanResults := Scan(ctx, providers, ScanOptions{
		Parallelism: e.cfg.Parallelism,
		SkipLists:   skipLists,
	})
	scanDuration := time.Since(scanStart)
	e.logger.Info("scan completed in %s", scanDuration)

	// Phase 2: Render scan summary.
	displayNames := buildDisplayNames(providers)
	summaryRows := ui.ScanSummaryRowsFromResults(scanResults, displayNames)
	ui.RenderScanSummaryTable(summaryRows)

	// Phase 3: Dry-run check.
	if opts.DryRun {
		fmt.Fprintln(out, "\n[dry-run] Stopping before execution.")
		return nil
	}

	// Count available/outdated providers.
	totalOutdated := 0
	for _, r := range scanResults {
		totalOutdated += len(r.Outdated)
	}
	if totalOutdated == 0 {
		fmt.Fprintln(out, "Everything is up to date.")
		return nil
	}

	// Phase 4: Confirm.
	msg := fmt.Sprintf("Update %d package(s) across %d provider(s)?", totalOutdated, len(providers))
	if !ui.Confirm(msg, opts.Yes) {
		fmt.Fprintln(out, "Aborted.")
		return nil
	}

	// Phase 5: Execute.
	fmt.Fprintln(out, "\nRunning updates...")
	runStart := time.Now()
	progressIncrement := ui.ProgressBar(len(providers))

	// Wire verbose mode: tee provider subprocess output to console when --verbose.
	if opts.Verbose {
		provider.SetVerboseOutput(out)
		defer provider.SetVerboseOutput(nil)
	}

	updateResults := Execute(ctx, providers, scanResults, ExecuteOptions{
		Parallelism: e.cfg.Parallelism,
		OnComplete: func(name string, result provider.UpdateResult) {
			progressIncrement()
		},
	})

	totalDuration := time.Since(runStart)

	// Phase 6: Save state.
	e.state.LastRun = time.Now()
	e.state.DurationSeconds = totalDuration.Seconds()
	for name, result := range updateResults {
		errStr := (*string)(nil)
		if result.Error != nil {
			s := result.Error.Error()
			errStr = &s
		}
		status := "success"
		switch {
		case result.Error != nil && len(result.Updated) == 0:
			status = "failed"
		case len(result.Failed) > 0 || result.Error != nil:
			status = "partial"
		case len(result.Deferred) > 0 && len(result.Updated) == 0:
			status = "partial"
		}
		e.state.SetProviderResult(name, state.ProviderStatus{
			Status:          status,
			Updated:         result.Updated,
			Failed:          result.Failed,
			Deferred:        result.Deferred,
			Skipped:         result.Skipped,
			DurationSeconds: result.Duration.Seconds(),
			Error:           errStr,
			Timestamp:       time.Now(),
		})

		// Wire deferred cask state: if brew-cask deferred casks, store script path.
		if name == "brew-cask" && len(result.Deferred) > 0 {
			scriptPath, pathErr := provider.DeferredScriptPath()
			if pathErr != nil {
				e.logger.Warn("could not resolve deferred cask script path: %v", pathErr)
			} else {
				e.state.Deferred = state.DeferredState{
					Casks:  result.Deferred,
					Script: scriptPath,
				}
			}
		}
	}
	if saveErr := e.state.Save(); saveErr != nil {
		e.logger.Warn("failed to save state: %v", saveErr)
	}

	// Phase 7: Report.
	reports := BuildReport(providers, scanResults, updateResults)
	PrintReport(out, reports, totalDuration)

	return nil
}

// runDeferred executes the deferred cask script from the last run.
func (e *Engine) runDeferred(ctx context.Context, out io.Writer, opts Options) error {
	deferred := e.state.GetDeferred()
	if len(deferred.Casks) == 0 && deferred.Script == "" {
		fmt.Fprintln(out, "No deferred cask updates found.")
		return nil
	}
	if deferred.Script == "" {
		fmt.Fprintln(out, "No deferred cask script found.")
		return nil
	}
	// Execute the deferred script.
	// Always resolve the canonical script path rather than blindly trusting the state value.
	canonicalScript, pathErr := provider.DeferredScriptPath()
	if pathErr != nil {
		return fmt.Errorf("resolving deferred script path: %w", pathErr)
	}
	if deferred.Script != "" && deferred.Script != canonicalScript {
		e.logger.Warn("state deferred.script %q does not match canonical path %q; using canonical", deferred.Script, canonicalScript)
	}
	fmt.Fprintf(out, "Running deferred cask script: %s\n", canonicalScript)
	stdout, stderr, err := provider.RunCommand(ctx, "bash", canonicalScript)
	if err != nil {
		fmt.Fprintf(out, "Deferred script error: %v\nStderr: %s\n", err, stderr)
		return err
	}
	fmt.Fprintln(out, stdout)
	// Clear the deferred state.
	e.state.Deferred = state.DeferredState{}
	return e.state.Save()
}

// resolveProviders returns the providers to run based on opts.
func (e *Engine) resolveProviders(opts Options) ([]provider.Provider, error) {
	if len(opts.Providers) > 0 {
		if len(opts.ProviderNames) > 0 {
			return filterProvidersByName(opts.Providers, opts.ProviderNames), nil
		}
		return opts.Providers, nil
	}
	return nil, nil
}

// filterProvidersByName returns the subset of providers whose names are in names.
func filterProvidersByName(providers []provider.Provider, names []string) []provider.Provider {
	set := make(map[string]bool, len(names))
	for _, n := range names {
		set[n] = true
	}
	var filtered []provider.Provider
	for _, p := range providers {
		if set[p.Name()] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// buildSkipLists converts config skip lists into a map[providerName]map[pkg]bool.
func buildSkipLists(cfg *config.Config) map[string]map[string]bool {
	result := make(map[string]map[string]bool)
	addSkips := func(name string, list []string) {
		if len(list) == 0 {
			return
		}
		m := make(map[string]bool, len(list))
		for _, s := range list {
			m[s] = true
		}
		result[name] = m
	}
	addSkips("brew", cfg.Providers.Brew.Skip)
	addSkips("brew-cask", cfg.Providers.BrewCask.Skip)
	addSkips("npm", cfg.Providers.Npm.Skip)
	return result
}

// buildDisplayNames returns a map from provider name → display name.
func buildDisplayNames(providers []provider.Provider) map[string]string {
	m := make(map[string]string, len(providers))
	for _, p := range providers {
		m[p.Name()] = p.DisplayName()
	}
	return m
}
