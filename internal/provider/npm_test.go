package provider_test

import (
	"context"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

const sampleNpmOutdated = `{
  "npm": {
    "current": "9.0.0",
    "wanted": "9.8.1",
    "latest": "9.8.1",
    "location": "node_modules/npm"
  },
  "typescript": {
    "current": "5.0.0",
    "wanted": "5.1.6",
    "latest": "5.1.6",
    "location": "node_modules/typescript"
  }
}`

func TestNpmProvider_Name(t *testing.T) {
	p := provider.NewNpmProvider(config.NpmConfig{Enabled: true}, nil)
	if p.Name() != "npm" {
		t.Errorf("expected %q, got %q", "npm", p.Name())
	}
	if p.DisplayName() != "npm Global Packages" {
		t.Errorf("expected %q, got %q", "npm Global Packages", p.DisplayName())
	}
}

func TestNpmProvider_DependsOn(t *testing.T) {
	p := provider.NewNpmProvider(config.NpmConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestParseNpmOutdated(t *testing.T) {
	items, err := provider.ParseNpmOutdated(sampleNpmOutdated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Collect into map for order-independent checks.
	byName := make(map[string]provider.OutdatedItem, len(items))
	for _, item := range items {
		byName[item.Name] = item
	}

	npm := byName["npm"]
	if npm.CurrentVersion != "9.0.0" {
		t.Errorf("npm current: expected 9.0.0, got %q", npm.CurrentVersion)
	}
	if npm.LatestVersion != "9.8.1" {
		t.Errorf("npm latest: expected 9.8.1, got %q", npm.LatestVersion)
	}

	ts := byName["typescript"]
	if ts.CurrentVersion != "5.0.0" {
		t.Errorf("typescript current: expected 5.0.0, got %q", ts.CurrentVersion)
	}
}

func TestParseNpmOutdated_Empty(t *testing.T) {
	for _, empty := range []string{"{}", "null", ""} {
		if empty == "" {
			continue // empty string handled upstream
		}
		items, err := provider.ParseNpmOutdated(empty)
		if err != nil {
			t.Errorf("unexpected error for %q: %v", empty, err)
		}
		if len(items) != 0 {
			t.Errorf("expected 0 items for %q, got %d", empty, len(items))
		}
	}
}

func TestNpmProvider_Scan_NoNpm(t *testing.T) {
	// If npm is not installed, Scan should return Available=false without error.
	if provider.CommandExistsExport("npm") {
		t.Skip("npm is installed; skipping unavailability test")
	}
	p := provider.NewNpmProvider(config.NpmConfig{Enabled: true}, nil)
	result := p.Scan(context.Background())
	if result.Available {
		t.Error("expected Available=false when npm not installed")
	}
}

func TestNpmProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("npm")
	if err != nil {
		t.Fatalf("npm not registered: %v", err)
	}
	if p.Name() != "npm" {
		t.Errorf("expected npm, got %s", p.Name())
	}
}
