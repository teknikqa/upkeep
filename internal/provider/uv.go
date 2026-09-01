package provider

import (
	"context"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// UvProvider implements the uv (Python package/tool manager) updater: it can
// update uv itself and upgrade all globally installed `uv tool` packages.
type UvProvider struct {
	cfg    config.UvConfig
	logger *logging.Logger

	// checkAvailable overrides the `uv` binary presence check for testing.
	// When nil, the real CommandExists("uv") is used.
	checkAvailable func() bool

	// runSelfUpdate overrides `uv self update` for testing. When nil, the
	// real command is run.
	runSelfUpdate func(ctx context.Context) (string, error)

	// runToolUpgrade overrides `uv tool upgrade --all` for testing. When nil,
	// the real command is run.
	runToolUpgrade func(ctx context.Context) (string, error)
}

// NewUvProvider creates a new uv provider.
func NewUvProvider(cfg config.UvConfig, logger *logging.Logger) *UvProvider {
	return &UvProvider{cfg: cfg, logger: logger}
}

func (p *UvProvider) Name() string        { return "uv" }
func (p *UvProvider) DisplayName() string { return "uv (Python)" }
func (p *UvProvider) DependsOn() []string { return nil }

// Scan checks whether uv is installed. uv has no `--outdated`/`--dry-run`
// query for `uv self` or `uv tool`, so both operations are surfaced as
// pseudo items that Update runs wholesale (like pip's pipx handling).
func (p *UvProvider) Scan(ctx context.Context) ScanResult {
	if !p.isAvailable() {
		return ScanResult{Available: false, Message: "uv not found"}
	}

	var items []OutdatedItem
	if p.cfg.SelfUpdate {
		items = append(items, OutdatedItem{Name: "uv (self)", LatestVersion: "self-update"})
	}
	if p.cfg.Tool {
		items = append(items, OutdatedItem{Name: "uv tool (all packages)", LatestVersion: "upgrade-all"})
	}

	return ScanResult{Available: true, Outdated: items}
}

// Update runs `uv self update` and/or `uv tool upgrade --all`.
func (p *UvProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	start := time.Now()
	if !p.isAvailable() {
		return UpdateResult{Duration: time.Since(start)}
	}

	var updated, failed, skipped []string

	if p.cfg.SelfUpdate {
		ReportProgress(ctx, "uv-self", PackageStarting)
		out, err := p.doSelfUpdate(ctx)
		switch {
		case err == nil:
			updated = append(updated, "uv-self")
			ReportProgress(ctx, "uv-self", PackageUpdated)
		case isUvManagedInstall(out):
			// uv was installed via brew/pip/pipx/etc.; self-update only works
			// for the standalone installer, so defer to that package manager.
			p.logf("uv self update skipped: managed install (%v)", err)
			skipped = append(skipped, "uv-self")
			ReportProgress(ctx, "uv-self", PackageSkipped)
		default:
			p.logf("uv self update error: %v\n%s", err, out)
			failed = append(failed, "uv-self")
			ReportProgress(ctx, "uv-self", PackageFailed)
		}
	}

	if p.cfg.Tool {
		ReportProgress(ctx, "uv-tools", PackageStarting)
		out, err := p.doToolUpgrade(ctx)
		if err != nil {
			p.logf("uv tool upgrade --all error: %v\n%s", err, out)
			failed = append(failed, "uv-tools")
			ReportProgress(ctx, "uv-tools", PackageFailed)
		} else {
			updated = append(updated, "uv-tools")
			ReportProgress(ctx, "uv-tools", PackageUpdated)
		}
	}

	return UpdateResult{
		Updated:  updated,
		Failed:   failed,
		Skipped:  skipped,
		Duration: time.Since(start),
	}
}

// isAvailable calls the provider's override if set, otherwise the real
// CommandExists("uv") check.
func (p *UvProvider) isAvailable() bool {
	if p.checkAvailable != nil {
		return p.checkAvailable()
	}
	return CommandExists("uv")
}

// doSelfUpdate calls the provider's override if set, otherwise runs the real
// `uv self update` command.
func (p *UvProvider) doSelfUpdate(ctx context.Context) (string, error) {
	if p.runSelfUpdate != nil {
		return p.runSelfUpdate(ctx)
	}
	return RunCommandWithLog(ctx, p.logger, "uv", "self", "update")
}

// doToolUpgrade calls the provider's override if set, otherwise runs the
// real `uv tool upgrade --all` command.
func (p *UvProvider) doToolUpgrade(ctx context.Context) (string, error) {
	if p.runToolUpgrade != nil {
		return p.runToolUpgrade(ctx)
	}
	return RunCommandWithLog(ctx, p.logger, "uv", "tool", "upgrade", "--all")
}

// isUvManagedInstall reports whether uv's self-update output indicates uv
// was installed via a package manager rather than the standalone installer,
// in which case `uv self update` refuses to run. e.g.:
//
//	error: uv was installed through an external package manager and cannot update itself.
func isUvManagedInstall(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "external package manager") && strings.Contains(lower, "cannot update itself")
}

func (p *UvProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[uv] "+format, args...)
	}
}

func init() {
	Register(NewUvProvider(config.UvConfig{
		Enabled:    true,
		SelfUpdate: true,
		Tool:       true,
	}, nil))
}
