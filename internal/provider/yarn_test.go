package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

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

func TestYarnProvider_Scan_NotAvailable(t *testing.T) {
	p := provider.NewYarnProvider(config.YarnConfig{Enabled: true}, nil)
	p.SetCheckAvailable(func() bool { return false })

	result := p.Scan(context.Background())
	if result.Available {
		t.Error("expected Available=false")
	}
}

func TestYarnProvider_Scan_Available(t *testing.T) {
	// yarn (classic) has no per-package outdated listing for global scope,
	// so Scan always surfaces exactly one pseudo item when available.
	p := provider.NewYarnProvider(config.YarnConfig{Enabled: true}, nil)
	p.SetCheckAvailable(func() bool { return true })

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

func TestYarnProvider_Update_NotAvailable(t *testing.T) {
	p := provider.NewYarnProvider(config.YarnConfig{Enabled: true}, nil)
	p.SetCheckAvailable(func() bool { return false })
	p.SetRunUpgrade(func(_ context.Context) (string, error) {
		t.Fatal("upgrade should not run when yarn is unavailable")
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
			p.SetCheckAvailable(func() bool { return true })
			p.SetRunUpgrade(func(_ context.Context) (string, error) {
				return "", tt.err
			})

			result := p.Update(context.Background(), nil)

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
