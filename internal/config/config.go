// Package config handles loading and validating upkeep configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultConfigPath returns the default config file path.
func DefaultConfigPath() string {
	return expandHome("~/.config/upkeep/config.yaml")
}

// Config is the top-level configuration structure.
type Config struct {
	Parallelism   int                 `yaml:"parallelism"`
	Providers     ProvidersConfig     `yaml:"providers"`
	Notifications NotificationsConfig `yaml:"notifications"`
	Logging       LoggingConfig       `yaml:"logging"`
}

// ProvidersConfig holds per-provider configuration.
type ProvidersConfig struct {
	Brew       BrewConfig       `yaml:"brew"`
	BrewCask   BrewCaskConfig   `yaml:"brew-cask"`
	Npm        NpmConfig        `yaml:"npm"`
	Composer   ComposerConfig   `yaml:"composer"`
	Pip        PipConfig        `yaml:"pip"`
	Rust       RustConfig       `yaml:"rust"`
	VSCode     VSCodeConfig     `yaml:"vscode"`
	Omz        OmzConfig        `yaml:"omz"`
	Vim        VimConfig        `yaml:"vim"`
	Vagrant    VagrantConfig    `yaml:"vagrant"`
	VirtualBox VirtualBoxConfig `yaml:"virtualbox"`
}

// BrewConfig configures the Homebrew formulae provider.
type BrewConfig struct {
	Enabled   bool     `yaml:"enabled"`
	Skip      []string `yaml:"skip"`
	PostHooks []string `yaml:"post_hooks"`
}

// BrewCaskConfig configures the Homebrew cask provider.
type BrewCaskConfig struct {
	Enabled         bool            `yaml:"enabled"`
	Greedy          bool            `yaml:"greedy"`
	Skip            []string        `yaml:"skip"`
	AuthOverrides   map[string]bool `yaml:"auth_overrides"`
	RebuildOpenWith bool            `yaml:"rebuild_open_with"`
	AuthStrategy    string          `yaml:"auth_strategy"` // "defer" | "force-interactive" | "skip"
}

// NpmConfig configures the npm provider.
type NpmConfig struct {
	Enabled bool     `yaml:"enabled"`
	Skip    []string `yaml:"skip"`
}

// ComposerConfig configures the Composer provider.
type ComposerConfig struct {
	Enabled bool `yaml:"enabled"`
}

// PipConfig configures the pip/pipx provider.
type PipConfig struct {
	Enabled           bool `yaml:"enabled"`
	UpgradePip        bool `yaml:"upgrade_pip"`
	UpgradeSetuptools bool `yaml:"upgrade_setuptools"`
	Pipx              bool `yaml:"pipx"`
}

// RustConfig configures the Rust provider.
type RustConfig struct {
	Enabled            bool `yaml:"enabled"`
	Rustup             bool `yaml:"rustup"`
	CargoInstallUpdate bool `yaml:"cargo_install_update"`
}

// VSCodeConfig configures the VS Code / multi-editor provider.
type VSCodeConfig struct {
	Enabled bool     `yaml:"enabled"`
	Editors []string `yaml:"editors"`
	Timeout int      `yaml:"timeout"` // seconds per editor
}

// OmzConfig configures the Oh My Zsh provider.
type OmzConfig struct {
	Enabled bool `yaml:"enabled"`
}

// VimConfig configures the Vim provider.
type VimConfig struct {
	Enabled      bool   `yaml:"enabled"`
	UpdateScript string `yaml:"update_script"`
	PathogenDir  string `yaml:"pathogen_dir"`
	BundlesDir   string `yaml:"bundles_dir"`
}

// VagrantConfig configures the Vagrant provider.
type VagrantConfig struct {
	Enabled bool `yaml:"enabled"`
	Notify  bool `yaml:"notify"`
}

// VirtualBoxConfig configures the VirtualBox provider.
type VirtualBoxConfig struct {
	Enabled bool `yaml:"enabled"`
	Notify  bool `yaml:"notify"`
}

// NotificationsConfig configures macOS notifications.
type NotificationsConfig struct {
	Enabled bool   `yaml:"enabled"`
	Tool    string `yaml:"tool"` // "terminal-notifier" | "osascript"
}

// LoggingConfig configures log output.
type LoggingConfig struct {
	Dir   string `yaml:"dir"`
	Level string `yaml:"level"` // "debug" | "info" | "warn" | "error"
}

// defaults returns a Config populated with sensible defaults.
func defaults() *Config {
	return &Config{
		Parallelism: 4,
		Providers: ProvidersConfig{
			Brew: BrewConfig{
				Enabled: true,
				PostHooks: []string{
					"brew doctor --quiet",
					"brew autoremove --quiet",
					"brew cleanup --quiet",
				},
			},
			BrewCask: BrewCaskConfig{
				Enabled:         true,
				Greedy:          true,
				RebuildOpenWith: true,
				AuthStrategy:    "defer",
				AuthOverrides:   map[string]bool{},
			},
			Npm:      NpmConfig{Enabled: true},
			Composer: ComposerConfig{Enabled: true},
			Pip: PipConfig{
				Enabled:           true,
				UpgradePip:        true,
				UpgradeSetuptools: true,
				Pipx:              true,
			},
			Rust: RustConfig{
				Enabled:            true,
				Rustup:             true,
				CargoInstallUpdate: true,
			},
			VSCode: VSCodeConfig{
				Enabled: true,
				Editors: []string{"code", "cursor", "kiro", "windsurf", "agy"},
				Timeout: 300,
			},
			Omz: OmzConfig{Enabled: true},
			Vim: VimConfig{
				Enabled:      true,
				UpdateScript: "~/bin/update-vim.sh",
				PathogenDir:  "~/.vim/autoload",
				BundlesDir:   "~/.vim/bundle",
			},
			Vagrant:    VagrantConfig{Enabled: true, Notify: true},
			VirtualBox: VirtualBoxConfig{Enabled: true, Notify: true},
		},
		Notifications: NotificationsConfig{
			Enabled: true,
			Tool:    "terminal-notifier",
		},
		Logging: LoggingConfig{
			Dir:   "~/Library/Logs/upkeep",
			Level: "info",
		},
	}
}

// Load reads a config file from path and returns a Config.
// If path is empty or the file does not exist, defaults are returned.
// If the file exists but is invalid YAML, an error is returned.
func Load(path string) (*Config, error) {
	cfg := defaults()

	if path == "" {
		path = DefaultConfigPath()
	}

	path = expandHome(path)

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Still expand home in defaults before returning.
			expandHomePaths(cfg)
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config file %q: %w", path, err)
	}

	// Unmarshal into the defaults struct so unset fields keep their defaults.
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file %q: %w", path, err)
	}

	// Expand home in path fields.
	expandHomePaths(cfg)

	if err := validate(cfg); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

// expandHomePaths expands ~ in all path-valued config fields.
func expandHomePaths(cfg *Config) {
	cfg.Logging.Dir = expandHome(cfg.Logging.Dir)
	cfg.Providers.Vim.UpdateScript = expandHome(cfg.Providers.Vim.UpdateScript)
	cfg.Providers.Vim.PathogenDir = expandHome(cfg.Providers.Vim.PathogenDir)
	cfg.Providers.Vim.BundlesDir = expandHome(cfg.Providers.Vim.BundlesDir)
}

// validate checks that config values are within acceptable ranges.
func validate(cfg *Config) error {
	if cfg.Parallelism < 1 {
		return fmt.Errorf("parallelism must be >= 1, got %d", cfg.Parallelism)
	}
	if cfg.Parallelism > 32 {
		return fmt.Errorf("parallelism must be <= 32, got %d", cfg.Parallelism)
	}
	switch cfg.Providers.BrewCask.AuthStrategy {
	case "defer", "force-interactive", "skip":
		// valid
	default:
		return fmt.Errorf("brew-cask auth_strategy must be one of: defer, force-interactive, skip; got %q", cfg.Providers.BrewCask.AuthStrategy)
	}
	switch cfg.Logging.Level {
	case "debug", "info", "warn", "error":
		// valid
	default:
		return fmt.Errorf("logging level must be one of: debug, info, warn, error; got %q", cfg.Logging.Level)
	}
	return nil
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
