package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
	"github.com/teknikqa/upkeep/internal/notify"
)

const virtualBoxLatestURL = "https://download.virtualbox.org/virtualbox/LATEST.TXT"
const virtualBoxDownloadURL = "https://www.virtualbox.org/wiki/Downloads"

// VirtualBoxProvider checks for VirtualBox updates (notification only).
type VirtualBoxProvider struct {
	cfg      config.VirtualBoxConfig
	notifCfg config.NotificationsConfig
	logger   *logging.Logger
}

// NewVirtualBoxProvider creates a new VirtualBox provider.
func NewVirtualBoxProvider(cfg config.VirtualBoxConfig, notifCfg config.NotificationsConfig, logger *logging.Logger) *VirtualBoxProvider {
	return &VirtualBoxProvider{cfg: cfg, notifCfg: notifCfg, logger: logger}
}

func (p *VirtualBoxProvider) Name() string        { return "virtualbox" }
func (p *VirtualBoxProvider) DisplayName() string { return "VirtualBox" }
func (p *VirtualBoxProvider) DependsOn() []string { return nil }

// Scan checks local VirtualBox version and fetches latest from virtualbox.org.
func (p *VirtualBoxProvider) Scan(ctx context.Context) ScanResult {
	if !CommandExists("VBoxManage") {
		return ScanResult{Available: false, Message: "VBoxManage not found"}
	}

	installed, err := p.getInstalledVersion(ctx)
	if err != nil || installed == "" {
		return ScanResult{Available: true, Message: "could not determine VirtualBox version"}
	}

	latest, err := p.fetchLatestVersion(ctx)
	if err != nil {
		p.logf("fetching latest VirtualBox version: %v", err)
		return ScanResult{Available: true, Message: "could not fetch latest VirtualBox version"}
	}

	if installed == latest {
		return ScanResult{Available: true, Outdated: nil}
	}

	return ScanResult{
		Available: true,
		Outdated: []OutdatedItem{
			{
				Name:           "virtualbox",
				CurrentVersion: installed,
				LatestVersion:  latest,
			},
		},
	}
}

// Update sends a macOS notification if VirtualBox is outdated (no auto-update).
func (p *VirtualBoxProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	start := time.Now()

	for _, item := range items {
		if item.Name == "virtualbox" && item.LatestVersion != item.CurrentVersion {
			if p.cfg.Notify {
				n := notify.New(p.notifCfg)
				msg := fmt.Sprintf("VirtualBox %s available (installed: %s)", item.LatestVersion, item.CurrentVersion)
				if err := n.Notify("VirtualBox", msg, virtualBoxDownloadURL); err != nil {
					p.logf("notification error: %v", err)
				}
			}
		}
	}

	// No auto-update; report as "skipped".
	skipped := make([]string, 0, len(items))
	for _, item := range items {
		skipped = append(skipped, item.Name)
	}

	return UpdateResult{
		Skipped:  skipped,
		Duration: time.Since(start),
	}
}

// getInstalledVersion returns the installed VirtualBox version via VBoxManage.
func (p *VirtualBoxProvider) getInstalledVersion(ctx context.Context) (string, error) {
	stdout, _, err := RunCommand(ctx, "VBoxManage", "--version")
	if err != nil {
		return "", err
	}
	return p.stripBuildSuffix(stdout), nil
}

// stripBuildSuffix strips the rXXXXX build suffix from a VirtualBox version string.
// E.g., "7.0.10r158379" → "7.0.10"
func (p *VirtualBoxProvider) stripBuildSuffix(version string) string {
	version = strings.TrimSpace(version)
	if idx := strings.IndexByte(version, 'r'); idx >= 0 {
		version = version[:idx]
	}
	return version
}

// fetchLatestVersion fetches the latest VirtualBox version from LATEST.TXT.
func (p *VirtualBoxProvider) fetchLatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, virtualBoxLatestURL, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("fetching %s: %w", virtualBoxLatestURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(body)), nil
}

func (p *VirtualBoxProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[virtualbox] "+format, args...)
	}
}

func init() {
	Register(NewVirtualBoxProvider(
		config.VirtualBoxConfig{Enabled: true, Notify: true},
		config.NotificationsConfig{Enabled: true, Tool: "terminal-notifier"},
		nil,
	))
}
