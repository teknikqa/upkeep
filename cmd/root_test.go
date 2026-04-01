package cmd

import (
	"context"
	"testing"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

// rootMockProvider is a minimal Provider double for root_test.go.
type rootMockProvider struct {
	name string
}

func (m *rootMockProvider) Name() string        { return m.name }
func (m *rootMockProvider) DisplayName() string { return m.name }
func (m *rootMockProvider) DependsOn() []string { return nil }
func (m *rootMockProvider) Scan(_ context.Context) provider.ScanResult {
	return provider.ScanResult{Available: true}
}
func (m *rootMockProvider) Update(_ context.Context, _ []provider.OutdatedItem) provider.UpdateResult {
	return provider.UpdateResult{Duration: time.Second}
}

func makeProviders(names ...string) []provider.Provider {
	ps := make([]provider.Provider, len(names))
	for i, n := range names {
		ps[i] = &rootMockProvider{name: n}
	}
	return ps
}

// --- filterEnabledProviders ---

func TestFilterEnabledProviders_AllEnabled(t *testing.T) {
	cfg := config.Defaults()
	// All providers default to enabled — ensure all pass through.
	providers := makeProviders("brew", "npm", "pip")
	result := filterEnabledProviders(providers, cfg)
	if len(result) != 3 {
		t.Errorf("expected 3 providers, got %d", len(result))
	}
}

func TestFilterEnabledProviders_SomeDisabled(t *testing.T) {
	cfg := config.Defaults()
	cfg.Providers.Brew.Enabled = false
	cfg.Providers.Npm.Enabled = false

	providers := makeProviders("brew", "npm", "pip")
	result := filterEnabledProviders(providers, cfg)
	if len(result) != 1 {
		t.Errorf("expected 1 provider (pip), got %d: %v", len(result), result)
	}
	if result[0].Name() != "pip" {
		t.Errorf("expected pip, got %q", result[0].Name())
	}
}

func TestFilterEnabledProviders_AllDisabled(t *testing.T) {
	cfg := config.Defaults()
	cfg.Providers.Brew.Enabled = false
	cfg.Providers.Npm.Enabled = false
	cfg.Providers.Pip.Enabled = false

	providers := makeProviders("brew", "npm", "pip")
	result := filterEnabledProviders(providers, cfg)
	if len(result) != 0 {
		t.Errorf("expected 0 providers, got %d", len(result))
	}
}

func TestFilterEnabledProviders_UnknownProviderIncluded(t *testing.T) {
	// Providers not in the enabled map default to included (!ok → true).
	cfg := config.Defaults()
	providers := makeProviders("unknown-provider-xyz")
	result := filterEnabledProviders(providers, cfg)
	if len(result) != 1 {
		t.Errorf("expected unknown provider to be included, got %d results", len(result))
	}
}

func TestFilterEnabledProviders_EmptyInput(t *testing.T) {
	cfg := config.Defaults()
	result := filterEnabledProviders(nil, cfg)
	if len(result) != 0 {
		t.Errorf("expected empty result for nil input, got %d", len(result))
	}
}
