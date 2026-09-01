package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

var yarnPseudoItem = []provider.OutdatedItem{{Name: "yarn global (all packages)", LatestVersion: "upgrade-all"}}

func TestYarnProvider_Name(t *testing.T) {
	p := provider.NewYarnProvider(config.YarnConfig{Enabled: true}, nil)
	if p.Name() != "yarn" {
		t.Errorf("expected %q, got %q", "yarn", p.Name())
	}
	if p.DisplayName() != "Yarn Global Packages" {
		t.Errorf("expected %q, got %q", "Yarn Global Packages", p.DisplayName())
	}
}

func TestYarnProvider_DependsOn(t *testing.T) {
	p := provider.NewYarnProvider(config.YarnConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestYarnProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("yarn")
	if err != nil {
		t.Fatalf("yarn not registered: %v", err)
	}
	if p.Name() != "yarn" {
		t.Errorf("expected yarn, got %s", p.Name())
	}
}

func TestIsYarnBerry(t *testing.T) {
	cases := []struct {
		name    string
		version string
		want    bool
	}{
		{"classic 1.22.22", "1.22.22", false},
		{"berry 4.5.0", "4.5.0", true},
		{"berry 2.0.0", "2.0.0", true},
		{"trailing newline", "1.22.22\n", false},
		{"empty", "", false},
		{"no dot", "not-a-version", false},
		{"non-numeric major", "v1.22.22", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := provider.IsYarnBerry(tc.version); got != tc.want {
				t.Errorf("IsYarnBerry(%q) = %v, want %v", tc.version, got, tc.want)
			}
		})
	}
}

func TestYarnProvider_Scan_NotAvailable(t *testing.T) {
	p := provider.NewYarnProvider(config.YarnConfig{Enabled: true}, nil)
	p.SetCheckAvailable(func() bool { return false })

	result := p.Scan(context.Background())
	if result.Available {
		t.Error("expected Available=false")
	}
}

func TestYarnProvider_Scan_Classic(t *testing.T) {
	// yarn Classic has no per-package outdated listing for global scope,
	// so Scan always surfaces exactly one pseudo item when available.
	p := provider.NewYarnProvider(config.YarnConfig{Enabled: true}, nil)
	p.SetCheckAvailable(func() bool { return true })
	p.SetRunVersion(func(_ context.Context) (string, error) { return "1.22.22", nil })

	result := p.Scan(context.Background())
	if !result.Available {
		t.Fatal("expected Available=true")
	}
	if len(result.Outdated) != 1 {
		t.Fatalf("expected 1 pseudo item, got %d: %+v", len(result.Outdated), result.Outdated)
	}
	if result.Outdated[0].Name != "yarn global (all packages)" {
		t.Errorf("expected pseudo item name, got %q", result.Outdated[0].Name)
	}
}

func TestYarnProvider_Scan_Berry(t *testing.T) {
	// Yarn Berry removed `yarn global` — Scan should skip with a message
	// rather than surface a pseudo item that would fail opaquely at Update.
	p := provider.NewYarnProvider(config.YarnConfig{Enabled: true}, nil)
	p.SetCheckAvailable(func() bool { return true })
	p.SetRunVersion(func(_ context.Context) (string, error) { return "4.5.0", nil })

	result := p.Scan(context.Background())
	if !result.Available {
		t.Fatal("expected Available=true")
	}
	if len(result.Outdated) != 0 {
		t.Errorf("expected 0 outdated (Berry unsupported), got %d: %+v", len(result.Outdated), result.Outdated)
	}
	if result.Message == "" {
		t.Error("expected a non-empty message explaining Berry is unsupported")
	}
}

func TestYarnProvider_Update_NoItems(t *testing.T) {
	// Scan is the sole gate (unavailable, or Berry active, both yield 0
	// items) — Update must not run doUpgrade when items is empty.
	p := provider.NewYarnProvider(config.YarnConfig{Enabled: true}, nil)
	p.SetRunUpgrade(func(_ context.Context) (string, error) {
		t.Fatal("upgrade should not run with no items")
		return "", nil
	})

	result := p.Update(context.Background(), nil)
	if len(result.Updated) != 0 || len(result.Failed) != 0 {
		t.Errorf("expected empty result, got %+v", result)
	}
}

func TestYarnProvider_Update(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantUpdated bool
		wantFailed  bool
	}{
		{name: "success", err: nil, wantUpdated: true},
		{name: "failure", err: errors.New("upgrade failed"), wantFailed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := provider.NewYarnProvider(config.YarnConfig{Enabled: true}, nil)
			p.SetRunUpgrade(func(_ context.Context) (string, error) {
				return "", tt.err
			})

			result := p.Update(context.Background(), yarnPseudoItem)

			gotUpdated := len(result.Updated) == 1 && result.Updated[0] == "yarn-global"
			gotFailed := len(result.Failed) == 1 && result.Failed[0] == "yarn-global"

			if gotUpdated != tt.wantUpdated {
				t.Errorf("updated: got %v, want %v (result=%+v)", gotUpdated, tt.wantUpdated, result)
			}
			if gotFailed != tt.wantFailed {
				t.Errorf("failed: got %v, want %v (result=%+v)", gotFailed, tt.wantFailed, result)
			}
		})
	}
}
