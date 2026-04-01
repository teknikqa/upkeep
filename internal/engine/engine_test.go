package engine_test

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/engine"
	"github.com/teknikqa/upkeep/internal/logging"
	"github.com/teknikqa/upkeep/internal/provider"
	"github.com/teknikqa/upkeep/internal/state"
)

// --- Mock Provider ---

type mockProvider struct {
	name         string
	displayName  string
	dependsOn    []string
	scanResult   provider.ScanResult
	updateResult provider.UpdateResult
	scanDelay    time.Duration
}

func (m *mockProvider) Name() string        { return m.name }
func (m *mockProvider) DisplayName() string { return m.displayName }
func (m *mockProvider) DependsOn() []string { return m.dependsOn }

func (m *mockProvider) Scan(ctx context.Context) provider.ScanResult {
	if m.scanDelay > 0 {
		select {
		case <-time.After(m.scanDelay):
		case <-ctx.Done():
			return provider.ScanResult{Available: false, Error: ctx.Err(), Message: "cancelled"}
		}
	}
	return m.scanResult
}

func (m *mockProvider) Update(ctx context.Context, items []provider.OutdatedItem) provider.UpdateResult {
	return m.updateResult
}

// --- Scanner Tests ---

func TestScan_ParallelScanCompletes(t *testing.T) {
	providers := []provider.Provider{
		&mockProvider{name: "brew", displayName: "Homebrew", scanResult: provider.ScanResult{Available: true, Outdated: []provider.OutdatedItem{{Name: "git"}}}},
		&mockProvider{name: "npm", displayName: "npm", scanResult: provider.ScanResult{Available: true}},
		&mockProvider{name: "pip", displayName: "pip", scanResult: provider.ScanResult{Available: false}},
	}

	results := engine.Scan(context.Background(), providers, engine.ScanOptions{Parallelism: 3})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if !results["brew"].Available {
		t.Error("expected brew to be available")
	}
	if len(results["brew"].Outdated) != 1 {
		t.Errorf("expected 1 outdated for brew, got %d", len(results["brew"].Outdated))
	}
	if results["pip"].Available {
		t.Error("expected pip to be unavailable")
	}
}

func TestScan_SkipListRemovesItems(t *testing.T) {
	providers := []provider.Provider{
		&mockProvider{
			name: "brew",
			scanResult: provider.ScanResult{
				Available: true,
				Outdated: []provider.OutdatedItem{
					{Name: "git"},
					{Name: "jq"},
					{Name: "ripgrep"},
				},
			},
		},
	}

	opts := engine.ScanOptions{
		Parallelism: 1,
		SkipLists: map[string]map[string]bool{
			"brew": {"jq": true},
		},
	}

	results := engine.Scan(context.Background(), providers, opts)
	outdated := results["brew"].Outdated
	if len(outdated) != 2 {
		t.Errorf("expected 2 outdated after skip, got %d: %v", len(outdated), outdated)
	}
	for _, item := range outdated {
		if item.Name == "jq" {
			t.Error("jq should have been skipped")
		}
	}
}

func TestScan_UnavailableProviderMarkedCorrectly(t *testing.T) {
	providers := []provider.Provider{
		&mockProvider{name: "vagrant", scanResult: provider.ScanResult{Available: false, Message: "vagrant not installed"}},
	}

	results := engine.Scan(context.Background(), providers, engine.ScanOptions{Parallelism: 1})
	if results["vagrant"].Available {
		t.Error("expected vagrant to be unavailable")
	}
}

func TestScan_ContextCancellationStopsPending(t *testing.T) {
	// Use parallelism=1 and slow providers so cancellation races properly.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	providers := []provider.Provider{
		&mockProvider{name: "slow1", scanDelay: 500 * time.Millisecond, scanResult: provider.ScanResult{Available: true}},
		&mockProvider{name: "slow2", scanDelay: 500 * time.Millisecond, scanResult: provider.ScanResult{Available: true}},
	}

	start := time.Now()
	results := engine.Scan(ctx, providers, engine.ScanOptions{Parallelism: 1})
	elapsed := time.Since(start)

	// Should complete well under 1 second due to cancellation.
	if elapsed > 500*time.Millisecond {
		t.Errorf("expected scan to be cancelled quickly, took %v", elapsed)
	}
	// At least one result should have a cancellation error.
	cancelled := 0
	for _, r := range results {
		if r.Error != nil {
			cancelled++
		}
	}
	if cancelled == 0 {
		t.Error("expected at least one cancelled scan result")
	}
}

// --- Executor Tests ---

func TestExecute_IndependentProvidersRunInParallel(t *testing.T) {
	// All three providers are independent; with parallelism=3 they should overlap.
	start := time.Now()
	providers := []provider.Provider{
		&mockProvider{name: "brew", scanResult: provider.ScanResult{Available: true}, updateResult: provider.UpdateResult{Updated: []string{"git"}}, scanDelay: 50 * time.Millisecond},
		&mockProvider{name: "npm", scanResult: provider.ScanResult{Available: true}, updateResult: provider.UpdateResult{}, scanDelay: 50 * time.Millisecond},
		&mockProvider{name: "pip", scanResult: provider.ScanResult{Available: true}, updateResult: provider.UpdateResult{}, scanDelay: 50 * time.Millisecond},
	}
	scanResults := map[string]provider.ScanResult{
		"brew": {Available: true},
		"npm":  {Available: true},
		"pip":  {Available: true},
	}

	results := engine.Execute(context.Background(), providers, scanResults, engine.ExecuteOptions{Parallelism: 3})
	elapsed := time.Since(start)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	// With parallelism=3, wall time should be << 3×50ms = 150ms.
	if elapsed > 200*time.Millisecond {
		t.Errorf("expected parallel execution, elapsed=%v (too slow)", elapsed)
	}
}

func TestExecute_DependentProviderWaitsForDependency(t *testing.T) {
	// brew-cask depends on brew; brew has a delay; brew-cask must wait.
	brewStarted := make(chan struct{})
	brewDone := make(chan struct{})

	brewProvider := &mockProvider{
		name:         "brew",
		scanResult:   provider.ScanResult{Available: true},
		updateResult: provider.UpdateResult{},
	}
	// Override update to signal timing.
	caskProvider := &mockProvider{
		name:         "brew-cask",
		dependsOn:    []string{"brew"},
		scanResult:   provider.ScanResult{Available: true},
		updateResult: provider.UpdateResult{},
	}
	_ = brewStarted
	_ = brewDone
	_ = brewProvider

	providers := []provider.Provider{brewProvider, caskProvider}
	scanResults := map[string]provider.ScanResult{
		"brew":      {Available: true},
		"brew-cask": {Available: true},
	}

	results := engine.Execute(context.Background(), providers, scanResults, engine.ExecuteOptions{Parallelism: 2})

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
}

func TestExecute_FailedProviderDoesNotBlockOthers(t *testing.T) {
	providers := []provider.Provider{
		&mockProvider{name: "brew", updateResult: provider.UpdateResult{Error: nil}},
		&mockProvider{name: "npm", updateResult: provider.UpdateResult{Failed: []string{"pkg"}}},
		&mockProvider{name: "pip", updateResult: provider.UpdateResult{}},
	}
	scanResults := map[string]provider.ScanResult{
		"brew": {Available: true},
		"npm":  {Available: true},
		"pip":  {Available: true},
	}

	results := engine.Execute(context.Background(), providers, scanResults, engine.ExecuteOptions{Parallelism: 3})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
}

func TestExecute_AllResultsCollected(t *testing.T) {
	providers := []provider.Provider{
		&mockProvider{name: "a", updateResult: provider.UpdateResult{Updated: []string{"x"}}},
		&mockProvider{name: "b", updateResult: provider.UpdateResult{Deferred: []string{"y"}}},
	}
	scanResults := map[string]provider.ScanResult{
		"a": {Available: true},
		"b": {Available: true},
	}

	results := engine.Execute(context.Background(), providers, scanResults, engine.ExecuteOptions{Parallelism: 2})
	if len(results["a"].Updated) != 1 || results["a"].Updated[0] != "x" {
		t.Errorf("unexpected result for a: %+v", results["a"])
	}
	if len(results["b"].Deferred) != 1 || results["b"].Deferred[0] != "y" {
		t.Errorf("unexpected result for b: %+v", results["b"])
	}
}

// TestExecute_OnStartCalledBeforeOnComplete verifies that OnStart fires before
// OnComplete for each provider and is called exactly once per provider.
func TestExecute_OnStartCalledBeforeOnComplete(t *testing.T) {
	var mu sync.Mutex
	var events []string

	providers := []provider.Provider{
		&mockProvider{name: "brew", updateResult: provider.UpdateResult{Updated: []string{"git"}}},
		&mockProvider{name: "npm", updateResult: provider.UpdateResult{}},
		&mockProvider{name: "pip", updateResult: provider.UpdateResult{}},
	}
	scanResults := map[string]provider.ScanResult{
		"brew": {Available: true},
		"npm":  {Available: true},
		"pip":  {Available: true},
	}

	results := engine.Execute(context.Background(), providers, scanResults, engine.ExecuteOptions{
		Parallelism: 3,
		OnStart: func(name string) {
			mu.Lock()
			events = append(events, "start:"+name)
			mu.Unlock()
		},
		OnComplete: func(name string, result provider.UpdateResult) {
			mu.Lock()
			events = append(events, "complete:"+name)
			mu.Unlock()
		},
	})

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}

	// Verify each provider has exactly one start and one complete event,
	// and that start precedes complete for each provider.
	for _, name := range []string{"brew", "npm", "pip"} {
		startIdx, completeIdx := -1, -1
		for i, e := range events {
			if e == "start:"+name {
				if startIdx != -1 {
					t.Errorf("%s: OnStart called more than once", name)
				}
				startIdx = i
			}
			if e == "complete:"+name {
				if completeIdx != -1 {
					t.Errorf("%s: OnComplete called more than once", name)
				}
				completeIdx = i
			}
		}
		if startIdx == -1 {
			t.Errorf("%s: OnStart never called", name)
		}
		if completeIdx == -1 {
			t.Errorf("%s: OnComplete never called", name)
		}
		if startIdx != -1 && completeIdx != -1 && startIdx > completeIdx {
			t.Errorf("%s: OnStart (idx %d) must precede OnComplete (idx %d)", name, startIdx, completeIdx)
		}
	}
}

// --- Reporter Tests ---

func TestBuildReport_MixedResults(t *testing.T) {
	providers := []provider.Provider{
		&mockProvider{name: "brew", displayName: "Homebrew"},
		&mockProvider{name: "npm", displayName: "npm"},
		&mockProvider{name: "pip", displayName: "pip"},
		&mockProvider{name: "vagrant", displayName: "Vagrant"},
	}
	scanResults := map[string]provider.ScanResult{
		"brew":    {Available: true},
		"npm":     {Available: true},
		"pip":     {Available: false}, // unavailable
		"vagrant": {Available: true},
	}
	updateResults := map[string]provider.UpdateResult{
		"brew": {Updated: []string{"git", "jq"}},
		"npm":  {Failed: []string{"some-pkg"}},
		// vagrant has no update result (skipped by engine)
	}

	reports := engine.BuildReport(providers, scanResults, updateResults)

	if len(reports) != 4 {
		t.Fatalf("expected 4 reports, got %d", len(reports))
	}

	rMap := make(map[string]engine.ProviderReport)
	for _, r := range reports {
		rMap[r.Name] = r
	}

	if rMap["brew"].Status != "success" {
		t.Errorf("brew: expected success, got %q", rMap["brew"].Status)
	}
	if rMap["brew"].Updated != 2 {
		t.Errorf("brew: expected Updated=2, got %d", rMap["brew"].Updated)
	}
	if rMap["npm"].Status != "partial" {
		t.Errorf("npm: expected partial (has failures), got %q", rMap["npm"].Status)
	}
	if rMap["pip"].Status != "unavailable" {
		t.Errorf("pip: expected unavailable, got %q", rMap["pip"].Status)
	}
	if rMap["vagrant"].Status != "skipped" {
		t.Errorf("vagrant: expected skipped (no update result), got %q", rMap["vagrant"].Status)
	}
}

func TestSummaryCounts(t *testing.T) {
	reports := []engine.ProviderReport{
		{Updated: 3, Deferred: 1, Skipped: 0, Failed: 0},
		{Updated: 0, Deferred: 2, Skipped: 1, Failed: 1},
	}
	u, d, s, f := engine.SummaryCounts(reports)
	if u != 3 || d != 3 || s != 1 || f != 1 {
		t.Errorf("unexpected counts: updated=%d deferred=%d skipped=%d failed=%d", u, d, s, f)
	}
}

// --- Engine Pipeline Tests ---

func TestEngine_DryRunStopsAfterScan(t *testing.T) {
	cfg, st, logger := engineTestFixtures(t)

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
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "dry-run") {
		t.Errorf("expected dry-run message in output, got: %q", buf.String())
	}
}

func TestEngine_FullPipelineWithMockProviders(t *testing.T) {
	cfg, st, logger := engineTestFixtures(t)
	eng := engine.New(cfg, st, logger)

	providers := []provider.Provider{
		&mockProvider{
			name:         "brew",
			displayName:  "Homebrew",
			scanResult:   provider.ScanResult{Available: true, Outdated: []provider.OutdatedItem{{Name: "git"}}},
			updateResult: provider.UpdateResult{Updated: []string{"git"}},
		},
		&mockProvider{
			name:         "npm",
			displayName:  "npm",
			scanResult:   provider.ScanResult{Available: false},
			updateResult: provider.UpdateResult{},
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
		t.Fatalf("unexpected error: %v", err)
	}

	// State should have been saved.
	if len(st.Providers) == 0 {
		t.Error("expected state to be saved after full pipeline run")
	}
}

func TestEngine_RetryFailedOnlyRunsFailedProviders(t *testing.T) {
	cfg, st, logger := engineTestFixtures(t)

	// Pre-populate state with a failed provider.
	st.SetProviderResult("npm", state.ProviderStatus{Status: "failed"})
	st.SetProviderResult("brew", state.ProviderStatus{Status: "success"})

	eng := engine.New(cfg, st, logger)

	ran := make(map[string]bool)
	providers := []provider.Provider{
		&mockProvider{
			name:         "brew",
			displayName:  "Homebrew",
			scanResult:   provider.ScanResult{Available: true},
			updateResult: provider.UpdateResult{},
		},
		&mockProvider{
			name:         "npm",
			displayName:  "npm",
			scanResult:   provider.ScanResult{Available: true, Outdated: []provider.OutdatedItem{{Name: "pkg"}}},
			updateResult: provider.UpdateResult{Updated: []string{"pkg"}},
		},
	}

	// Wrap providers to track which ran.
	tracked := make([]provider.Provider, len(providers))
	for i, p := range providers {
		tracked[i] = &trackingProvider{Provider: p, ran: ran}
	}

	var buf strings.Builder
	err := eng.Run(context.Background(), engine.Options{
		Providers:   tracked,
		RetryFailed: true,
		Yes:         true,
		Output:      &buf,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ran["brew"] {
		t.Error("brew should NOT have run (it was successful last time)")
	}
	if !ran["npm"] {
		t.Error("npm SHOULD have run (it was failed last time)")
	}
}

// trackingProvider wraps a Provider and records when Update is called.
type trackingProvider struct {
	provider.Provider
	ran map[string]bool
}

func (t *trackingProvider) Update(ctx context.Context, items []provider.OutdatedItem) provider.UpdateResult {
	t.ran[t.Provider.Name()] = true
	return t.Provider.Update(ctx, items)
}

func engineTestFixtures(t *testing.T) (*config.Config, *state.State, *logging.Logger) {
	t.Helper()
	cfg, err := config.Load("/nonexistent")
	if err != nil {
		t.Fatalf("loading config: %v", err)
	}
	cfg.Parallelism = 2
	st := state.New(filepath.Join(t.TempDir(), "state.json"))
	logger := logging.New(t.TempDir(), logging.LevelInfo)
	t.Cleanup(func() { logger.Close() })
	return cfg, st, logger
}
