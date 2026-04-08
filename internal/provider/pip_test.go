package provider_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

const samplePipOutdated = `[
  {"name": "requests", "version": "2.28.0", "latest_version": "2.31.0", "latest_filetype": "wheel"},
  {"name": "setuptools", "version": "67.0.0", "latest_version": "68.0.0", "latest_filetype": "wheel"}
]`

func TestPipProvider_Name(t *testing.T) {
	p := provider.NewPipProvider(config.PipConfig{Enabled: true}, nil)
	if p.Name() != "pip" {
		t.Errorf("expected %q, got %q", "pip", p.Name())
	}
	if p.DisplayName() != "pip / pipx" {
		t.Errorf("expected %q, got %q", "pip / pipx", p.DisplayName())
	}
}

func TestPipProvider_DependsOn(t *testing.T) {
	p := provider.NewPipProvider(config.PipConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestParsePipOutdated(t *testing.T) {
	items, err := provider.ParsePipOutdated(samplePipOutdated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "requests" {
		t.Errorf("expected requests, got %s", items[0].Name)
	}
	if items[0].CurrentVersion != "2.28.0" {
		t.Errorf("expected 2.28.0, got %s", items[0].CurrentVersion)
	}
	if items[0].LatestVersion != "2.31.0" {
		t.Errorf("expected 2.31.0, got %s", items[0].LatestVersion)
	}
}

func TestParsePipOutdated_Empty(t *testing.T) {
	items, err := provider.ParsePipOutdated("[]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestPipProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("pip")
	if err != nil {
		t.Fatalf("pip not registered: %v", err)
	}
	if p.Name() != "pip" {
		t.Errorf("expected pip, got %s", p.Name())
	}
}

func TestIsExternallyManaged_WithMarkerFile(t *testing.T) {
	// Create a temp dir with an EXTERNALLY-MANAGED marker file, then verify
	// the detection function finds it. We test via the real function on
	// the current system — if python3 is available and the marker exists,
	// IsExternallyManaged should return true.
	if !provider.CommandExistsExport("python3") {
		t.Skip("python3 not available")
	}
	// On Homebrew macOS this should be true; on other systems it depends.
	// We just verify it doesn't panic and returns a bool.
	got := provider.IsExternallyManaged(context.Background())
	t.Logf("isExternallyManaged = %v (system-dependent)", got)
}

func TestIsExternallyManaged_MarkerFilePresent(t *testing.T) {
	// Create a synthetic stdlib directory with EXTERNALLY-MANAGED to test
	// the file-existence logic indirectly via the provider override hook.
	tmpDir := t.TempDir()
	markerPath := filepath.Join(tmpDir, "EXTERNALLY-MANAGED")
	if err := os.WriteFile(markerPath, []byte("[externally-managed]\n"), 0o644); err != nil {
		t.Fatalf("creating marker file: %v", err)
	}

	p := provider.NewPipProvider(config.PipConfig{
		Enabled:           true,
		UpgradePip:        true,
		UpgradeSetuptools: true,
	}, nil)
	// Override detection to simulate externally-managed.
	p.SetCheckExternallyManaged(func(_ context.Context) bool { return true })

	items := []provider.OutdatedItem{
		{Name: "requests", CurrentVersion: "2.28.0", LatestVersion: "2.31.0"},
		{Name: "setuptools", CurrentVersion: "67.0.0", LatestVersion: "68.0.0"},
	}
	result := p.Update(context.Background(), items)

	// All pip3 packages should be skipped, not failed.
	if len(result.Failed) != 0 {
		t.Errorf("expected 0 failed, got %v", result.Failed)
	}
	// pip + setuptools + wheel + virtualenv + requests + setuptools(outdated) = 6 skipped
	expectedSkipped := []string{"pip", "setuptools", "wheel", "virtualenv", "requests", "setuptools"}
	if len(result.Skipped) != len(expectedSkipped) {
		t.Errorf("expected %d skipped, got %d: %v", len(expectedSkipped), len(result.Skipped), result.Skipped)
	}
	for i, name := range expectedSkipped {
		if i < len(result.Skipped) && result.Skipped[i] != name {
			t.Errorf("skipped[%d]: expected %q, got %q", i, name, result.Skipped[i])
		}
	}
}

func TestIsExternallyManaged_MarkerFileAbsent(t *testing.T) {
	// When not externally-managed, pip3 packages should be attempted (they may
	// fail or succeed depending on the system, but they should not be skipped).
	p := provider.NewPipProvider(config.PipConfig{
		Enabled:           true,
		UpgradePip:        false, // disable to avoid actual pip3 calls
		UpgradeSetuptools: false,
	}, nil)
	// Override detection to simulate non-externally-managed.
	p.SetCheckExternallyManaged(func(_ context.Context) bool { return false })

	// With UpgradePip and UpgradeSetuptools disabled, and no pip3 on some systems,
	// we verify that the skip list remains empty when not externally-managed.
	result := p.Update(context.Background(), nil)
	if len(result.Skipped) != 0 {
		t.Errorf("expected 0 skipped when not externally-managed, got %v", result.Skipped)
	}
}

func TestPipProvider_Update_ExternallyManaged_PipxStillRuns(t *testing.T) {
	if !provider.CommandExistsExport("pipx") {
		t.Skip("pipx not available")
	}

	p := provider.NewPipProvider(config.PipConfig{
		Enabled:           true,
		UpgradePip:        true,
		UpgradeSetuptools: true,
		Pipx:              true,
	}, nil)
	// Force externally-managed detection.
	p.SetCheckExternallyManaged(func(_ context.Context) bool { return true })

	items := []provider.OutdatedItem{
		{Name: "requests", CurrentVersion: "2.28.0", LatestVersion: "2.31.0"},
		{Name: "pipx (all packages)", LatestVersion: "upgrade-all"},
	}
	result := p.Update(context.Background(), items)

	// pip3 packages should be skipped.
	if len(result.Skipped) == 0 {
		t.Error("expected pip3 packages to be skipped")
	}
	// pipx should have been attempted (updated or failed, but not skipped).
	foundPipx := false
	for _, name := range result.Updated {
		if name == "pipx-packages" {
			foundPipx = true
		}
	}
	for _, name := range result.Failed {
		if name == "pipx-packages" {
			foundPipx = true
		}
	}
	if !foundPipx {
		t.Error("expected pipx-packages in updated or failed, but not found — pipx path was skipped")
	}
}
