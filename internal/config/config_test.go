package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
)

func TestLoad_MissingFileReturnsDefaults(t *testing.T) {
	cfg, err := config.Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if cfg.Parallelism != 4 {
		t.Errorf("expected default parallelism=4, got %d", cfg.Parallelism)
	}
	if !cfg.Providers.Brew.Enabled {
		t.Error("expected brew enabled by default")
	}
	if cfg.Providers.BrewCask.AuthStrategy != "defer" {
		t.Errorf("expected default auth_strategy=defer, got %q", cfg.Providers.BrewCask.AuthStrategy)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default log level=info, got %q", cfg.Logging.Level)
	}
}

func TestLoad_ValidYAML(t *testing.T) {
	yaml := `
parallelism: 2
providers:
  brew:
    enabled: false
    skip:
      - "some-formula"
  npm:
    enabled: true
logging:
  level: "debug"
`
	f := writeTempFile(t, yaml)
	cfg, err := config.Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Parallelism != 2 {
		t.Errorf("expected parallelism=2, got %d", cfg.Parallelism)
	}
	if cfg.Providers.Brew.Enabled {
		t.Error("expected brew disabled")
	}
	if len(cfg.Providers.Brew.Skip) != 1 || cfg.Providers.Brew.Skip[0] != "some-formula" {
		t.Errorf("expected skip=[some-formula], got %v", cfg.Providers.Brew.Skip)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level=debug, got %q", cfg.Logging.Level)
	}
}

func TestLoad_InvalidYAMLReturnsError(t *testing.T) {
	f := writeTempFile(t, "parallelism: [not an int}")
	_, err := config.Load(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestLoad_HomeExpansion(t *testing.T) {
	cfg, err := config.Load("/nonexistent/path/config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "Library/Logs/upkeep")
	if cfg.Logging.Dir != expected {
		t.Errorf("expected logging dir %q, got %q", expected, cfg.Logging.Dir)
	}
}

func TestLoad_SkipListsParse(t *testing.T) {
	yaml := `
providers:
  brew:
    skip:
      - "formula-a"
      - "formula-b"
  brew-cask:
    skip:
      - "cask-x"
`
	f := writeTempFile(t, yaml)
	cfg, err := config.Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Providers.Brew.Skip) != 2 {
		t.Errorf("expected 2 brew skip entries, got %d", len(cfg.Providers.Brew.Skip))
	}
	if len(cfg.Providers.BrewCask.Skip) != 1 || cfg.Providers.BrewCask.Skip[0] != "cask-x" {
		t.Errorf("expected brew-cask skip=[cask-x], got %v", cfg.Providers.BrewCask.Skip)
	}
}

func TestLoad_InvalidParallelism(t *testing.T) {
	f := writeTempFile(t, "parallelism: 0")
	_, err := config.Load(f)
	if err == nil {
		t.Fatal("expected error for parallelism=0, got nil")
	}
}

func TestLoad_InvalidAuthStrategy(t *testing.T) {
	yaml := `
providers:
  brew-cask:
    auth_strategy: "invalid"
`
	f := writeTempFile(t, yaml)
	_, err := config.Load(f)
	if err == nil {
		t.Fatal("expected error for invalid auth_strategy, got nil")
	}
}

func TestDefaults_ReturnsPopulatedConfig(t *testing.T) {
	cfg := config.Defaults()
	if cfg.Parallelism != 4 {
		t.Errorf("expected default parallelism=4, got %d", cfg.Parallelism)
	}
	if !cfg.Providers.Brew.Enabled {
		t.Error("expected brew enabled by default")
	}
	if cfg.Providers.BrewCask.AuthStrategy != "defer" {
		t.Errorf("expected default auth_strategy=defer, got %q", cfg.Providers.BrewCask.AuthStrategy)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default log level=info, got %q", cfg.Logging.Level)
	}
	if cfg.Notifications.Tool != "terminal-notifier" {
		t.Errorf("expected default notification tool=terminal-notifier, got %q", cfg.Notifications.Tool)
	}
	if len(cfg.Providers.VSCode.Editors) == 0 {
		t.Error("expected default VSCode editors to be non-empty")
	}
}

func TestValidate_ValidConfig(t *testing.T) {
	cfg := config.Defaults()
	if err := config.Validate(cfg); err != nil {
		t.Fatalf("expected defaults to pass validation, got: %v", err)
	}
}

func TestValidate_InvalidParallelism(t *testing.T) {
	cfg := config.Defaults()
	cfg.Parallelism = 0
	if err := config.Validate(cfg); err == nil {
		t.Fatal("expected error for parallelism=0")
	}
	cfg.Parallelism = 33
	if err := config.Validate(cfg); err == nil {
		t.Fatal("expected error for parallelism=33")
	}
}

func TestValidate_InvalidAuthStrategy(t *testing.T) {
	cfg := config.Defaults()
	cfg.Providers.BrewCask.AuthStrategy = "bogus"
	if err := config.Validate(cfg); err == nil {
		t.Fatal("expected error for invalid auth_strategy")
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := config.Defaults()
	cfg.Logging.Level = "trace"
	if err := config.Validate(cfg); err == nil {
		t.Fatal("expected error for invalid log level")
	}
}

func TestValidate_InvalidNotificationTool(t *testing.T) {
	cfg := config.Defaults()
	cfg.Notifications.Tool = "growl"
	if err := config.Validate(cfg); err == nil {
		t.Fatal("expected error for invalid notification tool")
	}
}

func TestValidate_VSCodeMarketplace_Valid(t *testing.T) {
	cfg := config.Defaults()
	cfg.Providers.VSCode.Marketplace = map[string]string{
		"agy":    "vsmarketplace",
		"cursor": "openvsx",
	}
	if err := config.Validate(cfg); err != nil {
		t.Fatalf("unexpected error for valid marketplace overrides: %v", err)
	}
}

func TestValidate_VSCodeMarketplace_Invalid(t *testing.T) {
	cfg := config.Defaults()
	cfg.Providers.VSCode.Marketplace = map[string]string{
		"agy": "mymarketplace",
	}
	if err := config.Validate(cfg); err == nil {
		t.Fatal("expected error for invalid marketplace value")
	}
}

func TestValidate_VSCodeMarketplace_Nil(t *testing.T) {
	// nil Marketplace map (the default) should be valid.
	cfg := config.Defaults()
	cfg.Providers.VSCode.Marketplace = nil
	if err := config.Validate(cfg); err != nil {
		t.Fatalf("unexpected error with nil marketplace map: %v", err)
	}
}

func TestSave_WritesValidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := config.Defaults()
	cfg.Parallelism = 8
	cfg.Providers.Brew.Enabled = false

	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load failed after Save: %v", err)
	}
	if loaded.Parallelism != 8 {
		t.Errorf("expected parallelism=8, got %d", loaded.Parallelism)
	}
	if loaded.Providers.Brew.Enabled {
		t.Error("expected brew disabled after roundtrip")
	}
}

func TestSave_CreatesDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deep", "config.yaml")

	cfg := config.Defaults()
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected file to exist at %q, got: %v", path, err)
	}
}

func TestSave_RejectsInvalidConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	cfg := config.Defaults()
	cfg.Parallelism = 0

	err := config.Save(cfg, path)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}

	// File should NOT have been written.
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("expected no file to be written for invalid config")
	}
}

// writeTempFile creates a temp file with the given content and returns its path.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "upkeep-config-*.yaml")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	t.Cleanup(func() { os.Remove(f.Name()) })
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	f.Close()
	return f.Name()
}
