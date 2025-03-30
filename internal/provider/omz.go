package provider

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// OmzProvider implements the Oh My Zsh updater.
type OmzProvider struct {
	cfg    config.OmzConfig
	logger *logging.Logger
}

// NewOmzProvider creates a new Oh My Zsh provider.
func NewOmzProvider(cfg config.OmzConfig, logger *logging.Logger) *OmzProvider {
	return &OmzProvider{cfg: cfg, logger: logger}
}

func (p *OmzProvider) Name() string        { return "omz" }
func (p *OmzProvider) DisplayName() string { return "Oh My Zsh" }
func (p *OmzProvider) DependsOn() []string { return nil }

// omzDir returns the expanded path to ~/.oh-my-zsh.
func omzDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".oh-my-zsh")
}

// Scan checks if ~/.oh-my-zsh exists and whether remote has new commits.
func (p *OmzProvider) Scan(ctx context.Context) ScanResult {
	dir := omzDir()
	if dir == "" {
		return ScanResult{Available: false, Message: "could not determine home dir"}
	}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return ScanResult{Available: false, Message: "~/.oh-my-zsh not found"}
	}

	// Check if upstream has changes (non-destructive fetch --dry-run).
	_, _, err := RunCommand(ctx, "git", "-C", dir, "fetch", "--dry-run")
	if err != nil {
		// fetch --dry-run failure is non-fatal; report as possibly outdated.
		return ScanResult{
			Available: true,
			Outdated:  []OutdatedItem{{Name: "oh-my-zsh", LatestVersion: "unknown"}},
			Message:   "fetch check failed (assuming update available)",
		}
	}

	// Check if local is behind remote.
	stdout, _, _ := RunCommand(ctx, "git", "-C", dir, "rev-list", "--count", "HEAD..@{u}")
	behind := len(stdout) > 0 && stdout != "0\n" && stdout != "0"

	if !behind {
		return ScanResult{Available: true, Outdated: nil}
	}

	return ScanResult{
		Available: true,
		Outdated:  []OutdatedItem{{Name: "oh-my-zsh", LatestVersion: "upstream"}},
	}
}

// Update pulls the latest oh-my-zsh via git (shell-independent).
func (p *OmzProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	if len(items) == 0 {
		return UpdateResult{}
	}

	dir := omzDir()
	start := time.Now()

	out, err := RunCommandWithLog(ctx, p.logger, "git", "-C", dir, "pull", "--rebase", "origin", "master")
	if err != nil {
		p.logf("git pull error: %v\n%s", err, out)
		return UpdateResult{
			Failed:   []string{"oh-my-zsh"},
			Duration: time.Since(start),
		}
	}

	return UpdateResult{
		Updated:  []string{"oh-my-zsh"},
		Duration: time.Since(start),
	}
}

func (p *OmzProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[omz] "+format, args...)
	}
}

func init() {
	Register(NewOmzProvider(config.OmzConfig{Enabled: true}, nil))
}
