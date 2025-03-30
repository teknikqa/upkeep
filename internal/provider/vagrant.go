package provider

import (
	"context"
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
}

// NewVagrantProvider creates a new Vagrant provider.
func NewVagrantProvider(cfg config.VagrantConfig, notifCfg config.NotificationsConfig, logger *logging.Logger) *VagrantProvider {
	return &VagrantProvider{cfg: cfg, notifCfg: notifCfg, logger: logger}
}

func (p *VagrantProvider) Name() string        { return "vagrant" }
func (p *VagrantProvider) DisplayName() string { return "Vagrant" }
func (p *VagrantProvider) DependsOn() []string { return nil }

// Scan runs `vagrant version --machine-readable` to get installed and latest versions.
func (p *VagrantProvider) Scan(ctx context.Context) ScanResult {
	if !CommandExists("vagrant") {
		return ScanResult{Available: false, Message: "vagrant not found"}
	}

	stdout, _, err := RunCommand(ctx, "vagrant", "version", "--machine-readable")
	if err != nil {
		// Fall back to just installed version.
		installed, iErr := p.getInstalledVersion(ctx)
		if iErr != nil {
			return ScanResult{Available: true, Message: "could not determine vagrant version"}
		}
		return ScanResult{
			Available: true,
			Outdated:  []OutdatedItem{{Name: "vagrant-plugins", CurrentVersion: installed}},
		}
	}

	installed, latest := parseVagrantVersion(stdout)
	var items []OutdatedItem

	if installed != "" && latest != "" && installed != latest {
		items = append(items, OutdatedItem{
			Name:           "vagrant",
			CurrentVersion: installed,
			LatestVersion:  latest,
		})
	}
	// Always report plugins as needing a check.
	items = append(items, OutdatedItem{Name: "vagrant-plugins", CurrentVersion: installed})

	return ScanResult{Available: true, Outdated: items}
}

// getInstalledVersion returns the installed vagrant version via `vagrant --version`.
func (p *VagrantProvider) getInstalledVersion(ctx context.Context) (string, error) {
	stdout, _, err := RunCommand(ctx, "vagrant", "--version")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(strings.TrimSpace(stdout))
	if len(fields) >= 2 {
		return fields[len(fields)-1], nil
	}
	return "", nil
}

// Update runs `vagrant plugin update` and sends notification if binary is outdated.
func (p *VagrantProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	start := time.Now()
	var updated, failed []string

	// Check for binary version notification.
	for _, item := range items {
		if item.Name == "vagrant" && item.LatestVersion != "" && item.CurrentVersion != item.LatestVersion {
			if p.cfg.Notify {
				n := notify.New(p.notifCfg)
				msg := "Vagrant " + item.LatestVersion + " available (installed: " + item.CurrentVersion + ")"
				if err := n.Notify("Vagrant", msg, "https://www.vagrantup.com/downloads.html"); err != nil {
					p.logf("notification error: %v", err)
				}
			}
			// Don't auto-update the binary.
		}
	}

	// Update plugins.
	out, err := RunCommandWithLog(ctx, p.logger, "vagrant", "plugin", "update")
	if err != nil {
		p.logf("vagrant plugin update error: %v\n%s", err, out)
		failed = append(failed, "vagrant-plugins")
	} else {
		updated = append(updated, "vagrant-plugins")
	}

	return UpdateResult{
		Updated:  updated,
		Failed:   failed,
		Duration: time.Since(start),
	}
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
