package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
	"github.com/teknikqa/upkeep/internal/notify"
)

// VagrantProvider checks for Vagrant binary updates (notification only) and updates plugins.
type VagrantProvider struct {
	cfg      config.VagrantConfig
	notifCfg config.NotificationsConfig
	logger   *logging.Logger

	// gemsURL is the RubyGems API base used to look up plugin versions.
	gemsURL string
}

// NewVagrantProvider creates a new Vagrant provider.
func NewVagrantProvider(cfg config.VagrantConfig, notifCfg config.NotificationsConfig, logger *logging.Logger) *VagrantProvider {
	return &VagrantProvider{cfg: cfg, notifCfg: notifCfg, logger: logger, gemsURL: "https://rubygems.org"}
}

func (p *VagrantProvider) Name() string        { return "vagrant" }
func (p *VagrantProvider) DisplayName() string { return "Vagrant" }
func (p *VagrantProvider) DependsOn() []string { return nil }

// Scan reports a newer Vagrant release (notification only) and each plugin
// whose latest RubyGems version differs from the installed one.
func (p *VagrantProvider) Scan(ctx context.Context) ScanResult {
	if !CommandExists("vagrant") {
		return ScanResult{Available: false, Message: "vagrant not found"}
	}

	var items []OutdatedItem
	if stdout, _, err := RunCommand(ctx, "vagrant", "version", "--machine-readable"); err == nil {
		installed, latest := parseVagrantVersion(stdout)
		if installed != "" && latest != "" && installed != latest {
			items = append(items, OutdatedItem{Name: "vagrant", CurrentVersion: installed, LatestVersion: latest})
		}
	}

	stdout, _, err := RunCommand(ctx, "vagrant", "plugin", "list")
	if err != nil {
		return ScanResult{Available: true, Outdated: items, Error: fmt.Errorf("vagrant plugin list: %w", err)}
	}
	for _, plugin := range parseVagrantPluginList(stdout) {
		latest, err := p.latestGemVersion(ctx, plugin.Name)
		if err != nil {
			// Plugins installed from a local gem or private source aren't on RubyGems.
			p.logf("skipping %s: %v", plugin.Name, err)
			continue
		}
		if latest != plugin.CurrentVersion {
			plugin.LatestVersion = latest
			items = append(items, plugin)
		}
	}

	return ScanResult{Available: true, Outdated: items}
}

// latestGemVersion returns the latest released version of a gem on RubyGems.
func (p *VagrantProvider) latestGemVersion(ctx context.Context, name string) (string, error) {
	url := p.gemsURL + "/api/v1/versions/" + name + "/latest.json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}
	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decoding %s: %w", url, err)
	}
	// RubyGems answers "unknown" for gems it doesn't host.
	if body.Version == "" || body.Version == "unknown" {
		return "", fmt.Errorf("not found on RubyGems")
	}
	return body.Version, nil
}

// Update sends a notification if the binary is outdated and runs
// `vagrant plugin update` for the outdated plugins.
func (p *VagrantProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	start := time.Now()
	var plugins []string

	for _, item := range items {
		if item.Name != "vagrant" {
			plugins = append(plugins, item.Name)
			continue
		}
		// Don't auto-update the binary; just tell the user.
		if p.cfg.Notify {
			n := notify.New(p.notifCfg)
			msg := "Vagrant " + item.LatestVersion + " available (installed: " + item.CurrentVersion + ")"
			if err := n.Notify("Vagrant", msg, "https://www.vagrantup.com/downloads.html"); err != nil {
				p.logf("notification error: %v", err)
			}
		}
	}

	result := UpdateResult{}
	if len(plugins) > 0 {
		for _, name := range plugins {
			ReportProgress(ctx, name, PackageStarting)
		}
		args := append([]string{"plugin", "update"}, plugins...)
		out, err := RunCommandWithLog(ctx, p.logger, "vagrant", args...)
		status := PackageUpdated
		if err != nil {
			p.logf("vagrant plugin update error: %v\n%s", err, out)
			status = PackageFailed
			result.Failed = plugins
		} else {
			result.Updated = plugins
		}
		for _, name := range plugins {
			ReportProgress(ctx, name, status)
		}
	}

	result.Duration = time.Since(start)
	return result
}

// parseVagrantPluginList parses `vagrant plugin list` output, e.g.
//
//	vagrant-vbguest (0.32.0, global)
//	  - Version Constraint: > 0
//	vagrant-share (2.0.0, system)
//
// System plugins ship with Vagrant itself and are skipped.
func parseVagrantPluginList(output string) []OutdatedItem {
	var items []OutdatedItem
	for _, line := range strings.Split(output, "\n") {
		name, rest, ok := strings.Cut(line, " (")
		if !ok || name == "" || strings.HasPrefix(name, " ") {
			continue
		}
		version, scope, _ := strings.Cut(strings.TrimSuffix(rest, ")"), ", ")
		if scope == "system" {
			continue
		}
		items = append(items, OutdatedItem{Name: name, CurrentVersion: version})
	}
	return items
}

// parseVagrantVersion parses `vagrant version --machine-readable` output.
// Lines look like: "1234567890,default,version-installed,2.3.4"
// and              "1234567890,default,version-latest,2.4.0"
func parseVagrantVersion(output string) (installed, latest string) {
	for _, line := range strings.Split(output, "\n") {
		parts := strings.Split(strings.TrimSpace(line), ",")
		if len(parts) < 4 {
			continue
		}
		switch parts[2] {
		case "version-installed":
			installed = parts[3]
		case "version-latest", "version-current":
			latest = parts[3]
		}
	}
	return
}

func (p *VagrantProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[vagrant] "+format, args...)
	}
}

func init() {
	Register(NewVagrantProvider(
		config.VagrantConfig{Enabled: true, Notify: true},
		config.NotificationsConfig{Enabled: true, Tool: "terminal-notifier"},
		nil,
	))
}
