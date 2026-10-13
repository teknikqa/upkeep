package provider_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
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

func TestRustScan(t *testing.T) {
	t.Run("nothing installed", func(t *testing.T) {
		isolatedPath(t)
		if res := provider.NewRustProvider(config.RustConfig{}, nil).Scan(context.Background()); res.Available {
			t.Errorf("expected unavailable, got %+v", res)
		}
	})
	t.Run("rustup only", func(t *testing.T) {
		bin := isolatedPath(t)
		fakeCommand(t, bin, "rustup", map[string]fakeResponse{"check": {stdout: sampleRustupCheck}})
		res := provider.NewRustProvider(config.RustConfig{}, nil).Scan(context.Background())
		if want := []string{"stable-aarch64-apple-darwin"}; !reflect.DeepEqual(itemNames(res.Outdated), want) {
			t.Errorf("got %v, want %v", itemNames(res.Outdated), want)
		}
	})
	t.Run("rustup and cargo-install-update", func(t *testing.T) {
		bin := isolatedPath(t)
		fakeCommand(t, bin, "rustup", map[string]fakeResponse{"check": {stdout: sampleRustupCheck}})
		fakeCommand(t, bin, "cargo", map[string]fakeResponse{"install-update -a --list": {stdout: sampleCargoInstallUpdateList}})
		fakeCommand(t, bin, "cargo-install-update", nil)
		res := provider.NewRustProvider(config.RustConfig{}, nil).Scan(context.Background())
		want := []string{"stable-aarch64-apple-darwin", "bat", "fd-find"}
		if !reflect.DeepEqual(itemNames(res.Outdated), want) {
			t.Errorf("got %v, want %v", itemNames(res.Outdated), want)
		}
	})
}

func TestRustUpdate(t *testing.T) {
	t.Run("rustup succeeds, cargo fails", func(t *testing.T) {
		bin := isolatedPath(t)
		fakeCommand(t, bin, "rustup", map[string]fakeResponse{"update": {}})
		fakeCommand(t, bin, "cargo", map[string]fakeResponse{"install-update -a": {stderr: "boom", exit: 1}})
		fakeCommand(t, bin, "cargo-install-update", nil)
		logger := logging.New(t.TempDir(), logging.ParseLevel("debug"))
		defer func() { _ = logger.Close() }()

		res := provider.NewRustProvider(config.RustConfig{Rustup: true, CargoInstallUpdate: true}, logger).
			Update(context.Background(), nil)
		if !reflect.DeepEqual(res.Updated, []string{"rustup-toolchains"}) || !reflect.DeepEqual(res.Failed, []string{"cargo-binaries"}) {
			t.Errorf("unexpected result %+v", res)
		}
	})
	t.Run("rustup fails, cargo succeeds", func(t *testing.T) {
		bin := isolatedPath(t)
		fakeCommand(t, bin, "rustup", map[string]fakeResponse{"update": {exit: 1}})
		fakeCommand(t, bin, "cargo", map[string]fakeResponse{"install-update -a": {}})
		fakeCommand(t, bin, "cargo-install-update", nil)

		res := provider.NewRustProvider(config.RustConfig{Rustup: true, CargoInstallUpdate: true}, nil).
			Update(context.Background(), nil)
		if !reflect.DeepEqual(res.Updated, []string{"cargo-binaries"}) || !reflect.DeepEqual(res.Failed, []string{"rustup-toolchains"}) {
			t.Errorf("unexpected result %+v", res)
		}
	})
	t.Run("disabled in config", func(t *testing.T) {
		bin := isolatedPath(t)
		fakeCommand(t, bin, "rustup", nil)
		res := provider.NewRustProvider(config.RustConfig{}, nil).Update(context.Background(), nil)
		if len(res.Updated)+len(res.Failed) != 0 {
			t.Errorf("unexpected result %+v", res)
		}
	})
}
