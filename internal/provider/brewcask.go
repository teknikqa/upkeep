package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
	"github.com/teknikqa/upkeep/internal/notify"
)

// brewCaskOutdatedV2 matches the cask section of `brew outdated --cask --json=v2`.
type brewCaskOutdatedV2 struct {
	Casks []brewCask `json:"casks"`
}

type brewCask struct {
	Name              string   `json:"name"`
	InstalledVersions []string `json:"installed_versions"`
	CurrentVersion    string   `json:"current_version"`
}

// BrewCaskProvider implements the Homebrew cask updater with auth partitioning.
type BrewCaskProvider struct {
	cfg      config.BrewCaskConfig
	notifCfg config.NotificationsConfig
	logger   *logging.Logger
}

// NewBrewCaskProvider creates a new Homebrew cask provider.
func NewBrewCaskProvider(cfg config.BrewCaskConfig, notifCfg config.NotificationsConfig, logger *logging.Logger) *BrewCaskProvider {
	return &BrewCaskProvider{cfg: cfg, notifCfg: notifCfg, logger: logger}
}

func (p *BrewCaskProvider) Name() string        { return "brew-cask" }
func (p *BrewCaskProvider) DisplayName() string { return "Homebrew Casks" }
func (p *BrewCaskProvider) DependsOn() []string { return []string{"brew"} }

// Scan runs `brew outdated --cask --greedy --json=v2` and determines auth requirements.
func (p *BrewCaskProvider) Scan(ctx context.Context) ScanResult {
	if !CommandExists("brew") {
		return ScanResult{Available: false, Message: "brew not found"}
	}

	args := []string{"outdated", "--cask", "--json=v2"}
	if p.cfg.Greedy {
		args = append(args, "--greedy")
	}

	stdout, _, err := RunCommand(ctx, "brew", args...)
	if err != nil {
		return ScanResult{
			Available: true,
			Error:     err,
			Message:   "brew outdated --cask failed",
		}
	}

	casks, parseErr := parseBrewCaskOutdated(stdout)
	if parseErr != nil {
		p.logf("parsing brew outdated --cask output: %v", parseErr)
		return ScanResult{Available: true, Error: parseErr}
	}

	// Determine auth requirements for each cask.
	items := make([]OutdatedItem, 0, len(casks))
	for _, c := range casks {
		installed := ""
		if len(c.InstalledVersions) > 0 {
			installed = c.InstalledVersions[0]
		}
		item := OutdatedItem{
			Name:           c.Name,
			CurrentVersion: installed,
			LatestVersion:  c.CurrentVersion,
		}
		item.AuthRequired = p.detectAuthRequired(ctx, c.Name)
		items = append(items, item)
	}

	return ScanResult{
		Available: true,
		Outdated:  items,
	}
}

// Update partitions casks into auth/no-auth groups and handles each per config strategy.
func (p *BrewCaskProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	if len(items) == 0 {
		return UpdateResult{}
	}

	start := time.Now()
	var updated, deferred, skipped, failed []string

	var noAuth, authReq []OutdatedItem
	for _, item := range items {
		if item.AuthRequired {
			authReq = append(authReq, item)
		} else {
			noAuth = append(noAuth, item)
		}
	}

	// Update non-auth casks with NONINTERACTIVE=1.
	for _, item := range noAuth {
		env := []string{"NONINTERACTIVE=1"}
		out, err := RunCommandEnvWithLog(ctx, p.logger, env, "brew", "upgrade", "--cask", item.Name)
		if err != nil {
			p.logf("brew upgrade --cask %s error: %v\n%s", item.Name, err, out)
			failed = append(failed, item.Name)
		} else {
			updated = append(updated, item.Name)
		}
	}

	// Handle auth-required casks per strategy.
	switch p.cfg.AuthStrategy {
	case "force-interactive":
		for _, item := range authReq {
			out, err := RunCommandWithLog(ctx, p.logger, "brew", "upgrade", "--cask", item.Name)
			if err != nil {
				p.logf("brew upgrade --cask %s (interactive) error: %v\n%s", item.Name, err, out)
				failed = append(failed, item.Name)
			} else {
				updated = append(updated, item.Name)
			}
		}
	case "skip":
		for _, item := range authReq {
			skipped = append(skipped, item.Name)
		}
	default: // "defer"
		for _, item := range authReq {
			deferred = append(deferred, item.Name)
		}
		if len(deferred) > 0 {
			if err := p.writeDeferredScript(deferred); err != nil {
				p.logf("writing deferred script: %v", err)
			}
			p.sendDeferredNotification(deferred)
		}
	}

	// Rebuild Open With menu if configured and any casks were updated.
	if p.cfg.RebuildOpenWith && len(updated) > 0 {
		p.rebuildOpenWith(ctx)
	}

	return UpdateResult{
		Updated:  updated,
		Deferred: deferred,
		Skipped:  skipped,
		Failed:   failed,
		Duration: time.Since(start),
	}
}

const lsregisterPath = "/System/Library/Frameworks/CoreServices.framework/Versions/A/Frameworks/LaunchServices.framework/Versions/A/Support/lsregister"

// rebuildOpenWith rebuilds the macOS "Open With" menu.
func (p *BrewCaskProvider) rebuildOpenWith(ctx context.Context) {
	if _, _, err := RunCommand(ctx, lsregisterPath, "-r", "-domain", "local", "-domain", "user"); err != nil {
		p.logf("lsregister error (non-fatal): %v", err)
	}
	if _, _, err := RunCommand(ctx, "killall", "Finder"); err != nil {
		p.logf("killall Finder error (non-fatal): %v", err)
	}
}

// DeferredScriptPath returns the path to the deferred cask update script.
func DeferredScriptPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "state", "upkeep", "deferred-cask.sh"), nil
}

// writeDeferredScript writes a shell script that updates the deferred casks.
func (p *BrewCaskProvider) writeDeferredScript(casks []string) error {
	path, err := DeferredScriptPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating deferred script dir: %w", err)
	}

	content := p.buildDeferredScriptContent(casks)
	if err := os.WriteFile(path, []byte(content), 0o700); err != nil {
		return fmt.Errorf("writing deferred script: %w", err)
	}
	return nil
}

// buildDeferredScriptContent returns the shell script content for deferred casks.
func (p *BrewCaskProvider) buildDeferredScriptContent(casks []string) string {
	var sb strings.Builder
	sb.WriteString("#!/bin/bash\n")
	sb.WriteString("set -euo pipefail\n")
	sb.WriteString(`export PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"` + "\n\n")
	sb.WriteString("# Deferred brew cask updates requiring admin authentication\n")
	sb.WriteString("# Generated by upkeep\n\n")
	for _, name := range casks {
		sb.WriteString(fmt.Sprintf("brew upgrade --cask %q\n", name))
	}
	return sb.String()
}

// sendDeferredNotification sends a macOS notification about deferred casks.
func (p *BrewCaskProvider) sendDeferredNotification(casks []string) {
	n := notify.New(p.notifCfg)
	msg := fmt.Sprintf("%d cask(s) need admin auth: %s", len(casks), strings.Join(casks, ", "))
	if err := n.Notify("Homebrew Casks", msg, ""); err != nil {
		p.logf("sending deferred notification: %v", err)
	}
}

// parseBrewCaskOutdated parses the JSON output of `brew outdated --cask --json=v2`.
func parseBrewCaskOutdated(jsonStr string) ([]brewCask, error) {
	var result brewCaskOutdatedV2
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, err
	}
	return result.Casks, nil
}

func (p *BrewCaskProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[brew-cask] "+format, args...)
	}
}

// init registers the BrewCaskProvider with sensible defaults.
func init() {
	Register(NewBrewCaskProvider(
		config.BrewCaskConfig{
			Enabled:         true,
			Greedy:          true,
			RebuildOpenWith: true,
			AuthStrategy:    "defer",
			AuthOverrides:   map[string]bool{},
		},
		config.NotificationsConfig{Enabled: true, Tool: "terminal-notifier"},
		nil,
	))
}
