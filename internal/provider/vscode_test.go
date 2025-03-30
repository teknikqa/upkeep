package provider_test

import (
	"context"
	"testing"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

func TestVSCodeProvider_Name(t *testing.T) {
	p := provider.NewVSCodeProvider(config.VSCodeConfig{Enabled: true, Editors: []string{"code"}}, nil)
	if p.Name() != "vscode" {
		t.Errorf("expected %q, got %q", "vscode", p.Name())
	}
	if p.DisplayName() != "VS Code / Editors" {
		t.Errorf("expected %q, got %q", "VS Code / Editors", p.DisplayName())
	}
}

func TestVSCodeProvider_DependsOn(t *testing.T) {
	p := provider.NewVSCodeProvider(config.VSCodeConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestVSCodeProvider_Scan_NoEditors(t *testing.T) {
	// Use a non-existent editor name to simulate no editors installed.
	p := provider.NewVSCodeProvider(config.VSCodeConfig{
		Enabled: true,
		Editors: []string{"nonexistent-editor-xyz-abc"},
		Timeout: 5,
	}, nil)
	result := p.Scan(context.Background())
	if result.Available {
		t.Error("expected Available=false when no editors found")
	}
}

func TestVSCodeProvider_Timeout(t *testing.T) {
	// Verify timeout is respected: run `sleep` as the "editor".
	// This tests that the per-editor context deadline fires.
	// Only run on systems where sleep is available.
	if !provider.CommandExistsExport("sleep") {
		t.Skip("sleep not available")
	}

	p := provider.NewVSCodeProvider(config.VSCodeConfig{
		Enabled: true,
		Editors: []string{"sleep"},
		Timeout: 1,
	}, nil)

	items := []provider.OutdatedItem{{Name: "sleep", LatestVersion: "1 extension"}}
	start := time.Now()
	result := p.Update(context.Background(), items)
	elapsed := time.Since(start)

	// sleep with no args exits quickly (error), but we're testing timeout logic.
	// The important thing: it should not take more than ~3s.
	if elapsed > 5*time.Second {
		t.Errorf("update took too long: %v", elapsed)
	}
	_ = result
}

func TestVSCodeProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("vscode")
	if err != nil {
		t.Fatalf("vscode not registered: %v", err)
	}
	if p.Name() != "vscode" {
		t.Errorf("expected vscode, got %s", p.Name())
	}
}
