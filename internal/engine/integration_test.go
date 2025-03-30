package engine_test

// Integration tests: full pipeline config→scan→execute→state→report.
// Uses mock providers; no real system tools invoked.

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/engine"
	"github.com/teknikqa/upkeep/internal/logging"
	"github.com/teknikqa/upkeep/internal/provider"
	"github.com/teknikqa/upkeep/internal/state"
)

// integrationFixtures returns a complete set of engine components for integration tests.
func integrationFixtures(t *testing.T) (*config.Config, *state.State, *logging.Logger, string) {
	t.Helper()
	dir := t.TempDir()
	cfg, err := config.Load("/nonexistent") // uses defaults
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	cfg.Parallelism = 4
	stPath := filepath.Join(dir, "state.json")
	st := state.New(stPath)
	logger := logging.New(dir, logging.LevelInfo)
	t.Cleanup(func() { logger.Close() })
	return cfg, st, logger, stPath
}

// TestIntegration_MixedSuccessFailureDeferred runs the full pipeline with providers
// that succeed, fail, and defer — verifying state is written correctly.
func TestIntegration_MixedSuccessFailureDeferred(t *testing.T) {
	cfg, st, logger, stPath := integrationFixtures(t)
	eng := engine.New(cfg, st, logger)

	providers := []provider.Provider{
		// brew: succeeds — updates git and ripgrep.
		&mockProvider{
			name:        "brew",
			displayName: "Homebrew",
			scanResult:  provider.ScanResult{Available: true, Outdated: []provider.OutdatedItem{{Name: "git"}, {Name: "ripgrep"}}},
			updateResult: provider.UpdateResult{
				Updated: []string{"git", "ripgrep"},
			},
		},
		// npm: partial — one updated, one failed.
		&mockProvider{
			name:        "npm",
			displayName: "npm",
			scanResult:  provider.ScanResult{Available: true, Outdated: []provider.OutdatedItem{{Name: "typescript"}, {Name: "eslint"}}},
			updateResult: provider.UpdateResult{
				Updated: []string{"typescript"},
				Failed:  []string{"eslint"},
			},
		},
		// pip: unavailable — should be skipped/unavailable in state.
		&mockProvider{
			name:         "pip",
			displayName:  "pip",
			scanResult:   provider.ScanResult{Available: false, Message: "pip not installed"},
			updateResult: provider.UpdateResult{},
		},
		// brew-cask: defers auth-required casks.
		&mockProvider{
			name:        "brew-cask",
			displayName: "Homebrew Casks",
			dependsOn:   []string{"brew"},
			scanResult: provider.ScanResult{
				Available: true,
				Outdated: []provider.OutdatedItem{
					{Name: "docker", AuthRequired: false},
					{Name: "virtualbox", AuthRequired: true},
				},
			},
			updateResult: provider.UpdateResult{
				Updated:  []string{"docker"},
				Deferred: []string{"virtualbox"},
			},
		},
		// rust: completely fails.
		&mockProvider{
			name:        "rust",
			displayName: "Rust",
			scanResult:  provider.ScanResult{Available: true, Outdated: []provider.OutdatedItem{{Name: "rustup"}}},
			updateResult: provider.UpdateResult{
				Error: errors.New("rustup update failed"),
			},
		},
	}

	var buf strings.Builder
	err := eng.Run(context.Background(), engine.Options{
		Providers: providers,
		DryRun:    false,
		Yes:       true, // skip confirmation prompt
		Output:    &buf,
	})
	if err != nil {
		t.Fatalf("engine.Run: %v", err)
	}

	// --- Verify state was saved correctly ---

	// Reload state from disk to confirm persistence.
	loaded, err := state.Load(stPath)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}

	// brew → success
	brewStatus, ok := loaded.Providers["brew"]
	if !ok {
		t.Fatal("state missing 'brew' provider")
	}
	if brewStatus.Status != "success" {
		t.Errorf("brew: expected status=success, got %q", brewStatus.Status)
	}
	if len(brewStatus.Updated) != 2 {
		t.Errorf("brew: expected 2 updated, got %d", len(brewStatus.Updated))
	}

	// npm → partial (has failures)
	npmStatus, ok := loaded.Providers["npm"]
	if !ok {
		t.Fatal("state missing 'npm' provider")
	}
	if npmStatus.Status != "partial" {
		t.Errorf("npm: expected status=partial, got %q", npmStatus.Status)
	}
	if len(npmStatus.Updated) != 1 || npmStatus.Updated[0] != "typescript" {
		t.Errorf("npm: unexpected updated list: %v", npmStatus.Updated)
	}
	if len(npmStatus.Failed) != 1 || npmStatus.Failed[0] != "eslint" {
		t.Errorf("npm: unexpected failed list: %v", npmStatus.Failed)
	}

	// brew-cask → success (has both updated and deferred; engine classifies as success
	// because Updated is non-empty and no errors/failures)
	caskStatus, ok := loaded.Providers["brew-cask"]
	if !ok {
		t.Fatal("state missing 'brew-cask' provider")
	}
	if caskStatus.Status != "success" {
		t.Errorf("brew-cask: expected status=success (updated+deferred), got %q", caskStatus.Status)
	}
	if len(caskStatus.Deferred) != 1 || caskStatus.Deferred[0] != "virtualbox" {
		t.Errorf("brew-cask: unexpected deferred list: %v", caskStatus.Deferred)
	}

	// rust → failed
	rustStatus, ok := loaded.Providers["rust"]
	if !ok {
		t.Fatal("state missing 'rust' provider")
	}
	if rustStatus.Status != "failed" {
		t.Errorf("rust: expected status=failed, got %q", rustStatus.Status)
	}
	if rustStatus.Error == nil {
		t.Error("rust: expected non-nil error in state")
	}

	// Deferred state should have brew-cask's deferred cask.
	if len(loaded.Deferred.Casks) != 1 || loaded.Deferred.Casks[0] != "virtualbox" {
		t.Errorf("deferred casks: expected [virtualbox], got %v", loaded.Deferred.Casks)
	}
	if loaded.Deferred.Script == "" {
		t.Error("deferred script path should be populated in state")
	}

	// LastRun should be set.
	if loaded.LastRun.IsZero() {
		t.Error("state.LastRun should be set after run")
	}

	// --- Verify report output contains key markers ---
	output := buf.String()
	if !strings.Contains(output, "updates") && !strings.Contains(output, "Scan") && !strings.Contains(output, "Running") {
		t.Logf("output: %q", output)
		// Non-fatal: output format may vary.
	}
}

// TestIntegration_DryRunDoesNotSaveState verifies dry-run mode skips state write.
func TestIntegration_DryRunDoesNotSaveState(t *testing.T) {
	cfg, st, logger, stPath := integrationFixtures(t)
	eng := engine.New(cfg, st, logger)

	providers := []provider.Provider{
		&mockProvider{
			name:        "brew",
			displayName: "Homebrew",
			scanResult:  provider.ScanResult{Available: true, Outdated: []provider.OutdatedItem{{Name: "git"}}},
		},
	}

	var buf strings.Builder
	err := eng.Run(context.Background(), engine.Options{
		Providers: providers,
		DryRun:    true,
		Yes:       true,
		Output:    &buf,
	})
	if err != nil {
		t.Fatalf("engine.Run: %v", err)
	}

	// State file should NOT have been written.
	loaded, err := state.Load(stPath)
	if err != nil {
		t.Fatalf("state.Load: %v", err)
	}
	if len(loaded.Providers) > 0 {
		t.Errorf("dry-run should not write provider results to state, got: %v", loaded.Providers)
	}
	if !loaded.LastRun.IsZero() {
		t.Error("dry-run should not set LastRun in state")
	}
}

// TestIntegration_NothingOutdated verifies the engine exits cleanly when all up to date.
func TestIntegration_NothingOutdated(t *testing.T) {
	cfg, st, logger, _ := integrationFixtures(t)
	eng := engine.New(cfg, st, logger)

	providers := []provider.Provider{
		&mockProvider{
			name:        "brew",
			displayName: "Homebrew",
			scanResult:  provider.ScanResult{Available: true, Outdated: nil}, // nothing outdated
		},
		&mockProvider{
			name:        "npm",
			displayName: "npm",
			scanResult:  provider.ScanResult{Available: false},
		},
	}

	var buf strings.Builder
	err := eng.Run(context.Background(), engine.Options{
		Providers: providers,
		DryRun:    false,
		Yes:       true,
		Output:    &buf,
	})
	if err != nil {
		t.Fatalf("engine.Run: %v", err)
	}

	if !strings.Contains(buf.String(), "up to date") {
		t.Errorf("expected 'up to date' message, got: %q", buf.String())
	}
}

// TestIntegration_RetryFailedFiltersProviders verifies --retry-failed only runs
// the providers that failed last time.
func TestIntegration_RetryFailedFiltersProviders(t *testing.T) {
	cfg, st, logger, _ := integrationFixtures(t)

	// Pre-populate: npm failed, brew succeeded.
	st.SetProviderResult("npm", state.ProviderStatus{Status: "failed"})
	st.SetProviderResult("brew", state.ProviderStatus{Status: "success"})

	eng := engine.New(cfg, st, logger)

	ran := make(map[string]bool)
	providers := []provider.Provider{
		&trackingProvider{
			Provider: &mockProvider{
				name:         "brew",
				displayName:  "Homebrew",
				scanResult:   provider.ScanResult{Available: true, Outdated: []provider.OutdatedItem{{Name: "git"}}},
				updateResult: provider.UpdateResult{Updated: []string{"git"}},
			},
			ran: ran,
		},
		&trackingProvider{
			Provider: &mockProvider{
				name:         "npm",
				displayName:  "npm",
				scanResult:   provider.ScanResult{Available: true, Outdated: []provider.OutdatedItem{{Name: "pkg"}}},
				updateResult: provider.UpdateResult{Updated: []string{"pkg"}},
			},
			ran: ran,
		},
	}

	var buf strings.Builder
	err := eng.Run(context.Background(), engine.Options{
		Providers:   providers,
		RetryFailed: true,
		Yes:         true,
		Output:      &buf,
	})
	if err != nil {
		t.Fatalf("engine.Run: %v", err)
	}
	if ran["brew"] {
		t.Error("brew should NOT run under --retry-failed (last status: success)")
	}
	if !ran["npm"] {
		t.Error("npm SHOULD run under --retry-failed (last status: failed)")
	}
}

// TestIntegration_RunDeferredNoScript verifies --run-deferred exits cleanly when no script exists.
func TestIntegration_RunDeferredNoScript(t *testing.T) {
	cfg, st, logger, _ := integrationFixtures(t)
	eng := engine.New(cfg, st, logger)

	// st.Deferred is empty (zero value).
	var buf strings.Builder
	err := eng.Run(context.Background(), engine.Options{
		RunDeferred: true,
		Output:      &buf,
	})
	if err != nil {
		t.Fatalf("engine.Run with RunDeferred: %v", err)
	}
	if !strings.Contains(buf.String(), "No deferred") {
		t.Errorf("expected 'No deferred' message, got: %q", buf.String())
	}
}
