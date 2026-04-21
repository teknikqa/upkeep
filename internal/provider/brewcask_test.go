package provider_test

import (
	"context"
	"strings"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

const sampleCaskOutdated = `{
  "formulae": [],
  "casks": [
    {
      "name": "iterm2",
      "installed_versions": ["3.4.20"],
      "current_version": "3.4.22"
    },
    {
      "name": "docker",
      "installed_versions": ["20.10.0,20.10.0"],
      "current_version": "24.0.0,24.0.0"
    }
  ]
}`

func TestBrewCaskProvider_Name(t *testing.T) {
	p := provider.NewBrewCaskProvider(
		config.BrewCaskConfig{Enabled: true, Greedy: true, AuthStrategy: "defer", AuthOverrides: map[string]bool{}},
		config.NotificationsConfig{Enabled: false},
		nil,
	)
	if p.Name() != "brew-cask" {
		t.Errorf("expected name %q, got %q", "brew-cask", p.Name())
	}
	if p.DisplayName() != "Homebrew Casks" {
		t.Errorf("expected display name %q, got %q", "Homebrew Casks", p.DisplayName())
	}
}

func TestBrewCaskProvider_DependsOn(t *testing.T) {
	p := provider.NewBrewCaskProvider(
		config.BrewCaskConfig{Enabled: true, AuthStrategy: "defer", AuthOverrides: map[string]bool{}},
		config.NotificationsConfig{Enabled: false},
		nil,
	)
	deps := p.DependsOn()
	if len(deps) != 1 || deps[0] != "brew" {
		t.Errorf("expected DependsOn=[\"brew\"], got %v", deps)
	}
}

func TestParseBrewCaskOutdated(t *testing.T) {
	items, err := provider.ParseBrewCaskOutdated(sampleCaskOutdated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "iterm2" {
		t.Errorf("expected iterm2, got %s", items[0].Name)
	}
	if items[0].CurrentVersion != "3.4.20" {
		t.Errorf("expected installed version 3.4.20, got %s", items[0].CurrentVersion)
	}
	if items[0].LatestVersion != "3.4.22" {
		t.Errorf("expected latest version 3.4.22, got %s", items[0].LatestVersion)
	}
}

func TestBrewCaskProvider_AuthOverride(t *testing.T) {
	p := provider.NewBrewCaskProvider(
		config.BrewCaskConfig{
			Enabled:       true,
			AuthStrategy:  "defer",
			AuthOverrides: map[string]bool{"docker": true, "iterm2": false},
		},
		config.NotificationsConfig{Enabled: false},
		nil,
	)
	if !p.DetectAuthRequired(context.Background(), "docker") {
		t.Error("expected docker to require auth (config override)")
	}
	if p.DetectAuthRequired(context.Background(), "iterm2") {
		t.Error("expected iterm2 to NOT require auth (config override)")
	}
}

func TestBrewCaskProvider_Update_DeferStrategy(t *testing.T) {
	p := provider.NewBrewCaskProvider(
		config.BrewCaskConfig{
			Enabled:         true,
			AuthStrategy:    "defer",
			RebuildOpenWith: false,
			AuthOverrides:   map[string]bool{},
		},
		config.NotificationsConfig{Enabled: false},
		nil,
	)

	// Pass in items marked manually (no real brew available in tests).
	items := []provider.OutdatedItem{
		{Name: "iterm2", AuthRequired: false},
	}

	// Without brew, this will fail — but Update should still return a result,
	// not panic. We mainly test the deferred path doesn't panic.
	result := p.Update(context.Background(), items)
	// Either updated or failed — as long as no panic.
	total := len(result.Updated) + len(result.Failed) + len(result.Deferred) + len(result.Skipped)
	if total == 0 {
		t.Error("expected at least one item accounted for in result")
	}
}

func TestBrewCaskProvider_Update_SkipStrategy(t *testing.T) {
	p := provider.NewBrewCaskProvider(
		config.BrewCaskConfig{
			Enabled:         true,
			AuthStrategy:    "skip",
			RebuildOpenWith: false,
			AuthOverrides:   map[string]bool{},
		},
		config.NotificationsConfig{Enabled: false},
		nil,
	)

	items := []provider.OutdatedItem{
		{Name: "docker", AuthRequired: true},
	}
	result := p.Update(context.Background(), items)
	if len(result.Skipped) != 1 || result.Skipped[0] != "docker" {
		t.Errorf("expected docker to be skipped, got skipped=%v", result.Skipped)
	}
}

func TestBrewCaskProvider_Update_DeferredItems(t *testing.T) {
	p := provider.NewBrewCaskProvider(
		config.BrewCaskConfig{
			Enabled:         true,
			AuthStrategy:    "defer",
			RebuildOpenWith: false,
			AuthOverrides:   map[string]bool{},
		},
		config.NotificationsConfig{Enabled: false},
		nil,
	)

	items := []provider.OutdatedItem{
		{Name: "some-auth-cask", AuthRequired: true},
		{Name: "vagrant", AuthRequired: true},
	}
	result := p.Update(context.Background(), items)
	if len(result.Deferred) != 2 {
		t.Errorf("expected 2 deferred items, got %v", result.Deferred)
	}
}

func TestDeferredScriptContent(t *testing.T) {
	script := provider.BuildDeferredScript([]string{"docker", "vagrant"})
	if !strings.Contains(script, "brew upgrade --cask") {
		t.Error("deferred script should contain brew upgrade --cask")
	}
	if !strings.Contains(script, "docker") {
		t.Error("deferred script should contain docker")
	}
	if !strings.Contains(script, "vagrant") {
		t.Error("deferred script should contain vagrant")
	}
	if !strings.Contains(script, "#!/bin/bash") {
		t.Error("deferred script should have shebang")
	}
}

func TestBrewCaskProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("brew-cask")
	if err != nil {
		t.Fatalf("brew-cask not registered: %v", err)
	}
	if p.Name() != "brew-cask" {
		t.Errorf("expected brew-cask, got %s", p.Name())
	}
}
