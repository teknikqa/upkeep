package provider_test

import (
	"context"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

func TestOmzProvider_Name(t *testing.T) {
	p := provider.NewOmzProvider(config.OmzConfig{Enabled: true}, nil)
	if p.Name() != "omz" {
		t.Errorf("expected %q, got %q", "omz", p.Name())
	}
	if p.DisplayName() != "Oh My Zsh" {
		t.Errorf("expected %q, got %q", "Oh My Zsh", p.DisplayName())
	}
}

func TestOmzProvider_DependsOn(t *testing.T) {
	p := provider.NewOmzProvider(config.OmzConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestOmzProvider_Scan_NotInstalled(t *testing.T) {
	// The provider checks for ~/.oh-my-zsh — if it doesn't exist, should report unavailable.
	// We can't control the test machine's home dir, so we test the update path instead.
	p := provider.NewOmzProvider(config.OmzConfig{Enabled: true}, nil)
	// Just verify it doesn't panic.
	result := p.Scan(context.Background())
	// Available could be true or false depending on whether tester has oh-my-zsh.
	_ = result
}

func TestOmzProvider_Update_EmptyItems(t *testing.T) {
	p := provider.NewOmzProvider(config.OmzConfig{Enabled: true}, nil)
	result := p.Update(context.Background(), nil)
	if len(result.Updated)+len(result.Failed) > 0 {
		t.Errorf("expected empty result for no items, got %+v", result)
	}
}

func TestOmzProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("omz")
	if err != nil {
		t.Fatalf("omz not registered: %v", err)
	}
	if p.Name() != "omz" {
		t.Errorf("expected omz, got %s", p.Name())
	}
}
