package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

// samplePnpmOutdated is real `pnpm outdated -g --format json` output,
// captured against pnpm 11.24.0.
const samplePnpmOutdated = `{
  "cowsay": {
    "current": "1.5.0",
    "latest": "1.6.0",
    "wanted": "1.5.0",
    "isDeprecated": false,
    "dependencyType": "dependencies"
  },
  "lodash": {
    "current": "4.17.20",
    "latest": "4.18.1",
    "wanted": "4.17.20",
    "isDeprecated": false,
    "dependencyType": "dependencies"
  }
}`

// samplePnpmGlobalBinNotInPath is the real stderr from `pnpm outdated -g`
// (and `pnpm update -g`) before `pnpm setup` has been run, captured against
// pnpm 11.24.0.
const samplePnpmGlobalBinNotInPath = `[ERROR] The configured global bin directory "/Users/nickmathew/Library/pnpm/bin" is not in PATH
Run "pnpm setup" to update your shell configuration.`

func TestPnpmProvider_Name(t *testing.T) {
	p := provider.NewPnpmProvider(config.PnpmConfig{Enabled: true}, nil)
	if p.Name() != "pnpm" {
		t.Errorf("expected %q, got %q", "pnpm", p.Name())
	}
	if p.DisplayName() != "pnpm Global Packages" {
		t.Errorf("expected %q, got %q", "pnpm Global Packages", p.DisplayName())
	}
}

func TestPnpmProvider_DependsOn(t *testing.T) {
	p := provider.NewPnpmProvider(config.PnpmConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestPnpmProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("pnpm")
	if err != nil {
		t.Fatalf("pnpm not registered: %v", err)
	}
	if p.Name() != "pnpm" {
		t.Errorf("expected pnpm, got %s", p.Name())
	}
}

func TestParsePnpmOutdated(t *testing.T) {
	items, err := provider.ParsePnpmOutdated(samplePnpmOutdated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	names := map[string]provider.OutdatedItem{}
	for _, item := range items {
		names[item.Name] = item
	}
	cowsay, ok := names["cowsay"]
	if !ok {
		t.Fatal("expected cowsay in outdated list")
	}
	if cowsay.CurrentVersion != "1.5.0" || cowsay.LatestVersion != "1.6.0" {
		t.Errorf("cowsay: expected 1.5.0 -> 1.6.0, got %s -> %s", cowsay.CurrentVersion, cowsay.LatestVersion)
	}
}

func TestParsePnpmOutdated_Empty(t *testing.T) {
	items, err := provider.ParsePnpmOutdated("{}")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestIsPnpmGlobalBinNotInPath(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{"real error", samplePnpmGlobalBinNotInPath, true},
		{"unrelated error", "network request failed", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := provider.IsPnpmGlobalBinNotInPath(tc.output); got != tc.want {
				t.Errorf("IsPnpmGlobalBinNotInPath(%q) = %v, want %v", tc.output, got, tc.want)
			}
		})
	}
}

func TestPnpmProvider_Scan_NotAvailable(t *testing.T) {
	p := provider.NewPnpmProvider(config.PnpmConfig{Enabled: true}, nil)
	p.SetCheckAvailable(func() bool { return false })

	result := p.Scan(context.Background())
	if result.Available {
		t.Error("expected Available=false")
	}
}

func TestPnpmProvider_Scan_GlobalBinNotInPath(t *testing.T) {
	p := provider.NewPnpmProvider(config.PnpmConfig{Enabled: true}, nil)
	p.SetCheckAvailable(func() bool { return true })
	p.SetRunOutdated(func(_ context.Context) (string, string, error) {
		return "", samplePnpmGlobalBinNotInPath, errors.New("exit status 1")
	})

	result := p.Scan(context.Background())
	if !result.Available {
		t.Fatal("expected Available=true")
	}
	if len(result.Outdated) != 0 {
		t.Errorf("expected 0 outdated, got %d", len(result.Outdated))
	}
	if result.Message == "" {
		t.Error("expected a non-empty message")
	}
}

func TestPnpmProvider_Scan_Outdated(t *testing.T) {
	tests := []struct {
		name      string
		stdout    string
		wantCount int
	}{
		{"populated", samplePnpmOutdated, 2},
		{"empty object", "{}", 0},
		{"empty string", "", 0},
		{"null", "null", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := provider.NewPnpmProvider(config.PnpmConfig{Enabled: true}, nil)
			p.SetCheckAvailable(func() bool { return true })
			p.SetRunOutdated(func(_ context.Context) (string, string, error) {
				return tt.stdout, "", nil
			})

			result := p.Scan(context.Background())
			if !result.Available {
				t.Fatal("expected Available=true")
			}
			if len(result.Outdated) != tt.wantCount {
				t.Errorf("expected %d outdated, got %d", tt.wantCount, len(result.Outdated))
			}
		})
	}
}

func TestPnpmProvider_Scan_MalformedJSON(t *testing.T) {
	p := provider.NewPnpmProvider(config.PnpmConfig{Enabled: true}, nil)
	p.SetCheckAvailable(func() bool { return true })
	p.SetRunOutdated(func(_ context.Context) (string, string, error) {
		return "{ not valid json", "", nil
	})

	result := p.Scan(context.Background())
	if !result.Available {
		t.Fatal("expected Available=true")
	}
	if result.Error == nil {
		t.Error("expected a parse error")
	}
}

func TestPnpmProvider_Update_Empty(t *testing.T) {
	p := provider.NewPnpmProvider(config.PnpmConfig{Enabled: true}, nil)
	result := p.Update(context.Background(), nil)
	if len(result.Updated) != 0 || len(result.Failed) != 0 {
		t.Errorf("expected empty result, got %+v", result)
	}
}

func TestPnpmProvider_Update_Batch(t *testing.T) {
	p := provider.NewPnpmProvider(config.PnpmConfig{Enabled: true}, nil)
	var gotNames []string
	p.SetRunUpdate(func(_ context.Context, names []string) (string, error) {
		gotNames = append(gotNames, names...)
		return "", nil
	})

	items := []provider.OutdatedItem{
		{Name: "cowsay", CurrentVersion: "1.5.0", LatestVersion: "1.6.0"},
		{Name: "lodash", CurrentVersion: "4.17.20", LatestVersion: "4.18.1"},
	}
	result := p.Update(context.Background(), items)

	if len(result.Updated) != 2 {
		t.Errorf("expected 2 updated, got %v", result.Updated)
	}
	if len(result.Failed) != 0 {
		t.Errorf("expected 0 failed, got %v", result.Failed)
	}
	if len(gotNames) != 2 {
		t.Errorf("expected doUpdate to see 2 names, got %v", gotNames)
	}
}

func TestPnpmProvider_Update_BatchFailsFallsBackPerPackage(t *testing.T) {
	p := provider.NewPnpmProvider(config.PnpmConfig{Enabled: true}, nil)
	p.SetRunUpdate(func(_ context.Context, names []string) (string, error) {
		if len(names) > 1 {
			return "", errors.New("batch failed")
		}
		if names[0] == "bad-pkg" {
			return "", errors.New("not found")
		}
		return "", nil
	})

	items := []provider.OutdatedItem{
		{Name: "cowsay"},
		{Name: "bad-pkg"},
	}
	result := p.Update(context.Background(), items)

	if len(result.Updated) != 1 || result.Updated[0] != "cowsay" {
		t.Errorf("expected [cowsay] updated, got %v", result.Updated)
	}
	if len(result.Failed) != 1 || result.Failed[0] != "bad-pkg" {
		t.Errorf("expected [bad-pkg] failed, got %v", result.Failed)
	}
}
