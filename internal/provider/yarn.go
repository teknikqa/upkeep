package provider

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// YarnProvider implements the Yarn (classic, 1.x) global packages updater.
// Yarn Classic has no per-package outdated listing for global scope
// (`yarn global outdated` doesn't exist), so this is a bucket-update
// provider like pip's pipx handling — a single pseudo item surfaces the
// wholesale `yarn global upgrade`.
//
// Yarn Berry (2.x+) removed `yarn global` entirely (replaced by `yarn dlx`
// for one-off execution — there's no Berry equivalent of "list/upgrade all
// globally installed packages"). Verified: running `yarn global upgrade`
// against Berry doesn't fail cleanly — it crashes with a confusing
// "Internal Error" workspace-resolution stack trace, since Berry parses
// "global" as an unrecognized command differently than Classic's "unknown
// subcommand" error. Scan detects the active yarn's major version and skips
// with an explanatory message when Berry is active, rather than surfacing a
// pseudo item that would fail opaquely at Update time.
type YarnProvider struct {
	cfg    config.YarnConfig
	logger *logging.Logger

	// checkAvailable overrides the `yarn` binary presence check for testing.
	// When nil, the real CommandExists("yarn") is used.
	checkAvailable func() bool

	// runVersion overrides `yarn --version` for testing. When nil, the real
	// command is run.
	runVersion func(ctx context.Context) (string, error)

	// runUpgrade overrides `yarn global upgrade` for testing. When nil, the
	// real command is run.
	runUpgrade func(ctx context.Context) (string, error)
}

// NewYarnProvider creates a new Yarn provider.
func NewYarnProvider(cfg config.YarnConfig, logger *logging.Logger) *YarnProvider {
	return &YarnProvider{cfg: cfg, logger: logger}
}

func (p *YarnProvider) Name() string        { return "yarn" }
func (p *YarnProvider) DisplayName() string { return "Yarn Global Packages" }
func (p *YarnProvider) DependsOn() []string { return nil }

// Scan checks whether yarn is installed and whether it's Yarn Classic (the
// only variant that supports global package management). There's no way to
// list which global packages are outdated, so a single pseudo item stands
// in for "run yarn global upgrade".
func (p *YarnProvider) Scan(ctx context.Context) ScanResult {
	if !p.isAvailable() {
		return ScanResult{Available: false, Message: "yarn not found"}
	}

	version, _ := p.doVersion(ctx)
	if isYarnBerry(version) {
		return ScanResult{
			Available: true,
			Message:   "yarn: Berry (2.x+) detected — `yarn global` was removed; switch to Yarn Classic (1.x) for global package management, or use `yarn dlx` per-invocation",
		}
	}

	return ScanResult{
		Available: true,
		Outdated:  []OutdatedItem{{Name: "yarn global (all packages)", LatestVersion: "upgrade-all"}},
	}
}

// Update runs `yarn global upgrade`. Scan is the sole gate on whether this
// runs at all — it returns no items when yarn is unavailable or Berry is
// active, so an empty items list here means there's nothing to do.
func (p *YarnProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	if len(items) == 0 {
		return UpdateResult{}
	}

	start := time.Now()

	ReportProgress(ctx, "yarn-global", PackageStarting)
	out, err := p.doUpgrade(ctx)
	if err != nil {
		p.logf("yarn global upgrade error: %v\n%s", err, out)
		ReportProgress(ctx, "yarn-global", PackageFailed)
		return UpdateResult{Failed: []string{"yarn-global"}, Duration: time.Since(start)}
	}

	ReportProgress(ctx, "yarn-global", PackageUpdated)
	return UpdateResult{Updated: []string{"yarn-global"}, Duration: time.Since(start)}
}

// isAvailable calls the provider's override if set, otherwise the real
// CommandExists("yarn") check.
func (p *YarnProvider) isAvailable() bool {
	if p.checkAvailable != nil {
		return p.checkAvailable()
	}
	return CommandExists("yarn")
}

// doVersion calls the provider's override if set, otherwise runs the real
// `yarn --version` command.
func (p *YarnProvider) doVersion(ctx context.Context) (string, error) {
	if p.runVersion != nil {
		return p.runVersion(ctx)
	}
	stdout, _, err := RunCommand(ctx, "yarn", "--version")
	return stdout, err
}

// doUpgrade calls the provider's override if set, otherwise runs the real
// `yarn global upgrade` command.
func (p *YarnProvider) doUpgrade(ctx context.Context) (string, error) {
	if p.runUpgrade != nil {
		return p.runUpgrade(ctx)
	}
	return RunCommandWithLog(ctx, p.logger, "yarn", "global", "upgrade")
}

// isYarnBerry reports whether a `yarn --version` output (e.g. "1.22.22" or
// "4.5.0") is Yarn Berry (major version 2+). An unparseable version fails
// open (returns false) so a version-check hiccup doesn't block Classic use.
func isYarnBerry(version string) bool {
	major, _, ok := strings.Cut(strings.TrimSpace(version), ".")
	if !ok {
		return false
	}
	n, err := strconv.Atoi(major)
	if err != nil {
		return false
	}
	return n >= 2
}

func (p *YarnProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[yarn] "+format, args...)
	}
}

func init() {
	Register(NewYarnProvider(config.YarnConfig{Enabled: true}, nil))
}
