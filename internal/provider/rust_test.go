package provider_test

import (
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

const sampleRustupCheck = `stable-aarch64-apple-darwin - Update available : 1.69.0 -> 1.71.0
nightly-aarch64-apple-darwin - Up to date : 1.74.0-nightly
`

const sampleCargoInstallUpdateList = `Package              Installed  Latest    Needs update
bat                  0.23.0     0.24.0    Yes
ripgrep              13.0.0     13.0.0    No
fd-find              8.7.0      9.0.0     Yes
`

func TestRustProvider_Name(t *testing.T) {
	p := provider.NewRustProvider(config.RustConfig{Enabled: true}, nil)
	if p.Name() != "rust" {
		t.Errorf("expected %q, got %q", "rust", p.Name())
	}
	if p.DisplayName() != "Rust (rustup + cargo)" {
		t.Errorf("expected %q, got %q", "Rust (rustup + cargo)", p.DisplayName())
	}
}

func TestRustProvider_DependsOn(t *testing.T) {
	p := provider.NewRustProvider(config.RustConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestParseRustupCheck(t *testing.T) {
	items := provider.ParseRustupCheck(sampleRustupCheck)
	if len(items) != 1 {
		t.Fatalf("expected 1 outdated toolchain, got %d", len(items))
	}
	if items[0].Name != "stable-aarch64-apple-darwin" {
		t.Errorf("expected stable toolchain, got %s", items[0].Name)
	}
	if items[0].CurrentVersion != "1.69.0" {
		t.Errorf("expected current 1.69.0, got %s", items[0].CurrentVersion)
	}
	if items[0].LatestVersion != "1.71.0" {
		t.Errorf("expected latest 1.71.0, got %s", items[0].LatestVersion)
	}
}

func TestParseCargoInstallUpdateList(t *testing.T) {
	items := provider.ParseCargoInstallUpdateList(sampleCargoInstallUpdateList)
	if len(items) != 2 {
		t.Fatalf("expected 2 outdated cargos, got %d", len(items))
	}
	names := map[string]bool{}
	for _, item := range items {
		names[item.Name] = true
	}
	if !names["bat"] {
		t.Error("expected bat in outdated list")
	}
	if !names["fd-find"] {
		t.Error("expected fd-find in outdated list")
	}
	if names["ripgrep"] {
		t.Error("ripgrep should not be in outdated list (up to date)")
	}
}

func TestRustProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("rust")
	if err != nil {
		t.Fatalf("rust not registered: %v", err)
	}
	if p.Name() != "rust" {
		t.Errorf("expected rust, got %s", p.Name())
	}
}
