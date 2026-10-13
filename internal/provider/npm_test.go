package provider_test

import (
	"context"
	"reflect"
	"sort"
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

func TestNpmProvider_Update_Empty(t *testing.T) {
	p := provider.NewNpmProvider(config.NpmConfig{Enabled: true}, nil)
	result := p.Update(context.Background(), nil)
	if len(result.Updated) != 0 || len(result.Failed) != 0 {
		t.Errorf("expected empty result for nil items, got updated=%v failed=%v", result.Updated, result.Failed)
	}
}

func TestNpmProvider_Update_ItemsAccountedFor(t *testing.T) {
	p := provider.NewNpmProvider(config.NpmConfig{Enabled: true}, nil)
	items := []provider.OutdatedItem{
		{Name: "npm-nonexistent-pkg-aaa"},
		{Name: "npm-nonexistent-pkg-bbb"},
	}
	// These package names don't exist (and npm may be absent), so the batch and
	// per-item fallback both fail — but every item must be accounted for.
	result := p.Update(context.Background(), items)
	total := len(result.Updated) + len(result.Failed)
	if total != 2 {
		t.Errorf("expected 2 items accounted for, got updated=%v failed=%v", result.Updated, result.Failed)
	}
}

func TestNpmScan(t *testing.T) {
	tests := []struct {
		name      string
		outdated  fakeResponse
		wantNames []string
		wantErr   bool
	}{
		// npm exits 1 whenever something is outdated.
		{"outdated packages", fakeResponse{stdout: sampleNpmOutdated, exit: 1}, []string{"npm", "typescript"}, false},
		{"empty output", fakeResponse{}, nil, false},
		{"empty object", fakeResponse{stdout: "{}"}, nil, false},
		{"bad json", fakeResponse{stdout: "not json", exit: 1}, nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bin := isolatedPath(t)
			fakeCommand(t, bin, "npm", map[string]fakeResponse{"outdated -g --json": tt.outdated})

			res := provider.NewNpmProvider(config.NpmConfig{}, nil).Scan(context.Background())
			if !res.Available || (res.Error != nil) != tt.wantErr {
				t.Fatalf("unexpected result %+v", res)
			}
			got := itemNames(res.Outdated)
			sort.Strings(got)
			if len(got)+len(tt.wantNames) > 0 && !reflect.DeepEqual(got, tt.wantNames) {
				t.Errorf("got %v, want %v", got, tt.wantNames)
			}
		})
	}
}

func TestNpmScan_NotInstalled(t *testing.T) {
	isolatedPath(t)
	if res := provider.NewNpmProvider(config.NpmConfig{}, nil).Scan(context.Background()); res.Available {
		t.Errorf("expected unavailable, got %+v", res)
	}
}
