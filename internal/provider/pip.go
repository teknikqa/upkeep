package provider

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// pipOutdatedPackage matches an entry from `pip3 list --outdated --format=json`.
type pipOutdatedPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Latest  string `json:"latest_version"`
}

// PipProvider implements pip3 + pipx updater.
type PipProvider struct {
	cfg    config.PipConfig
	logger *logging.Logger

	// checkExternallyManaged overrides the default PEP 668 detection for testing.
	// When nil, the real isExternallyManaged function is used.
	checkExternallyManaged func(ctx context.Context) bool
}

// NewPipProvider creates a new pip provider.
func NewPipProvider(cfg config.PipConfig, logger *logging.Logger) *PipProvider {
	return &PipProvider{cfg: cfg, logger: logger}
}

func (p *PipProvider) Name() string        { return "pip" }
func (p *PipProvider) DisplayName() string { return "pip / pipx" }
func (p *PipProvider) DependsOn() []string { return nil }

// Scan runs `pip3 list --outdated --format=json` and checks for pipx availability.
func (p *PipProvider) Scan(ctx context.Context) ScanResult {
	pip3Exists := CommandExists("pip3")
	pipxExists := CommandExists("pipx")

	if !pip3Exists && !pipxExists {
		return ScanResult{Available: false, Message: "pip3 and pipx not found"}
	}

	var items []OutdatedItem
	var message string

	if pip3Exists {
		stdout, _, err := RunCommand(ctx, "pip3", "list", "--outdated", "--format=json")
		if err == nil && stdout != "" && stdout != "[]" {
			parsed, parseErr := parsePipOutdated(stdout)
			if parseErr != nil {
				p.logf("parsing pip3 list output: %v", parseErr)
			} else {
				items = append(items, parsed...)
			}
		}
		if p.isExternallyManagedEnv(ctx) {
			message = "pip3: externally-managed environment (PEP 668) — pip3 packages will be skipped"
		}
	}

	// Indicate pipx is available as a pseudo-item (no per-package scan needed).
	if pipxExists {
		items = append(items, OutdatedItem{
			Name:          "pipx (all packages)",
			LatestVersion: "upgrade-all",
		})
	}

	return ScanResult{Available: true, Outdated: items, Message: message}
}

// Update upgrades pip packages and/or runs pipx upgrade-all.
func (p *PipProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	start := time.Now()
	var updated, failed, skipped []string

	if CommandExists("pip3") {
		extManaged := p.isExternallyManagedEnv(ctx)
		if extManaged {
			p.logf("skipping pip3 upgrades: externally-managed environment (PEP 668)")
		}

		// Upgrade pip itself and common tooling.
		if p.cfg.UpgradePip {
			if extManaged {
				skipped = append(skipped, "pip")
				ReportProgress(ctx, "pip", PackageSkipped)
			} else {
				ReportProgress(ctx, "pip", PackageStarting)
				out, err := RunCommandWithLog(ctx, p.logger, "pip3", "install", "--upgrade", "pip")
				if err != nil {
					p.logf("pip3 upgrade pip error: %v\n%s", err, out)
					failed = append(failed, "pip")
					ReportProgress(ctx, "pip", PackageFailed)
				} else {
					updated = append(updated, "pip")
					ReportProgress(ctx, "pip", PackageUpdated)
				}
			}
		}
		if p.cfg.UpgradeSetuptools {
			if extManaged {
				skipped = append(skipped, "setuptools", "wheel", "virtualenv")
				ReportProgress(ctx, "setuptools", PackageSkipped)
			} else {
				ReportProgress(ctx, "setuptools", PackageStarting)
				out, err := RunCommandWithLog(ctx, p.logger, "pip3", "install", "--upgrade", "setuptools", "wheel", "virtualenv")
				if err != nil {
					p.logf("pip3 upgrade setuptools error: %v\n%s", err, out)
					failed = append(failed, "setuptools")
					ReportProgress(ctx, "setuptools", PackageFailed)
				} else {
					updated = append(updated, "setuptools", "wheel", "virtualenv")
					ReportProgress(ctx, "setuptools", PackageUpdated)
				}
			}
		}
		// Upgrade each outdated package (skip the pipx pseudo-item).
		for _, item := range items {
			if item.Name == "pipx (all packages)" {
				continue
			}
			if extManaged {
				skipped = append(skipped, item.Name)
				ReportProgress(ctx, item.Name, PackageSkipped)
				continue
			}
			ReportProgress(ctx, item.Name, PackageStarting)
			out, err := RunCommandWithLog(ctx, p.logger, "pip3", "install", "--upgrade", item.Name)
			if err != nil {
				p.logf("pip3 upgrade %s error: %v\n%s", item.Name, err, out)
				failed = append(failed, item.Name)
				ReportProgress(ctx, item.Name, PackageFailed)
			} else {
				updated = append(updated, item.Name)
				ReportProgress(ctx, item.Name, PackageUpdated)
			}
		}
	}

	if p.cfg.Pipx && CommandExists("pipx") {
		ReportProgress(ctx, "pipx (all packages)", PackageStarting)
		out, err := RunCommandWithLog(ctx, p.logger, "pipx", "upgrade-all")
		if err != nil {
			p.logf("pipx upgrade-all error: %v\n%s", err, out)
			failed = append(failed, "pipx-packages")
			ReportProgress(ctx, "pipx-packages", PackageFailed)
		} else {
			updated = append(updated, "pipx-packages")
			ReportProgress(ctx, "pipx-packages", PackageUpdated)
		}
	}

	return UpdateResult{
		Updated:  updated,
		Failed:   failed,
		Skipped:  skipped,
		Duration: time.Since(start),
	}
}

// parsePipOutdated parses the JSON output of `pip3 list --outdated --format=json`.
func parsePipOutdated(jsonStr string) ([]OutdatedItem, error) {
	var packages []pipOutdatedPackage
	if err := json.Unmarshal([]byte(jsonStr), &packages); err != nil {
		return nil, err
	}
	items := make([]OutdatedItem, 0, len(packages))
	for _, pkg := range packages {
		items = append(items, OutdatedItem{
			Name:           pkg.Name,
			CurrentVersion: pkg.Version,
			LatestVersion:  pkg.Latest,
		})
	}
	return items, nil
}

// isExternallyManaged returns true if the system Python is marked as
// externally-managed per PEP 668 (e.g. Homebrew Python on macOS).
// It queries python3 for the stdlib path and checks for the EXTERNALLY-MANAGED
// marker file.
func isExternallyManaged(ctx context.Context) bool {
	stdout, _, err := RunCommand(ctx, "python3", "-c", "import sysconfig; print(sysconfig.get_path('stdlib'))")
	if err != nil {
		return false
	}
	stdlibPath := strings.TrimSpace(stdout)
	if stdlibPath == "" {
		return false
	}
	markerPath := filepath.Join(stdlibPath, "EXTERNALLY-MANAGED")
	_, err = os.Stat(markerPath)
	return err == nil
}

// isExternallyManagedEnv calls the provider's override if set, otherwise the
// real detection function.
func (p *PipProvider) isExternallyManagedEnv(ctx context.Context) bool {
	if p.checkExternallyManaged != nil {
		return p.checkExternallyManaged(ctx)
	}
	return isExternallyManaged(ctx)
}

func (p *PipProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[pip] "+format, args...)
	}
}

func init() {
	Register(NewPipProvider(config.PipConfig{
		Enabled:           true,
		UpgradePip:        true,
		UpgradeSetuptools: true,
		Pipx:              true,
	}, nil))
}
