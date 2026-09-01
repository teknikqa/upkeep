package provider

import (
	"context"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// YarnProvider implements the Yarn (classic, 1.x) global packages updater.
// Yarn Classic has no per-package outdated listing for global scope
// (`yarn global outdated` doesn't exist), so this is a bucket-update
// provider like pip's pipx handling — a single pseudo item surfaces the
// wholesale `yarn global upgrade`.
type YarnProvider struct {
	cfg    config.YarnConfig
	logger *logging.Logger

	// checkAvailable overrides the `yarn` binary presence check for testing.
	// When nil, the real CommandExists("yarn") is used.
	checkAvailable func() bool

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

// Scan checks whether yarn is installed. There's no way to list which
// global packages are outdated, so a single pseudo item stands in for
// "run yarn global upgrade".
func (p *YarnProvider) Scan(ctx context.Context) ScanResult {
	if !p.isAvailable() {
		return ScanResult{Available: false, Message: "yarn not found"}
	}

	return ScanResult{
		Available: true,
		Outdated:  []OutdatedItem{{Name: "yarn global (all packages)", LatestVersion: "upgrade-all"}},
	}
}

// Update runs `yarn global upgrade`.
func (p *YarnProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	start := time.Now()
	if !p.isAvailable() {
		return UpdateResult{Duration: time.Since(start)}
	}

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

// doUpgrade calls the provider's override if set, otherwise runs the real
// `yarn global upgrade` command.
func (p *YarnProvider) doUpgrade(ctx context.Context) (string, error) {
	if p.runUpgrade != nil {
		return p.runUpgrade(ctx)
	}
	return RunCommandWithLog(ctx, p.logger, "yarn", "global", "upgrade")
}

func (p *YarnProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[yarn] "+format, args...)
	}
}

func init() {
	Register(NewYarnProvider(config.YarnConfig{Enabled: true}, nil))
}
