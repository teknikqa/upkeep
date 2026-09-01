package provider

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// pnpmPackageInfo matches an entry from `pnpm outdated -g --format json`.
type pnpmPackageInfo struct {
	Current string `json:"current"`
	Latest  string `json:"latest"`
	Wanted  string `json:"wanted"`
}

// PnpmProvider implements the pnpm global packages updater.
type PnpmProvider struct {
	cfg    config.PnpmConfig
	logger *logging.Logger

	// checkAvailable overrides the `pnpm` binary presence check for testing.
	// When nil, the real CommandExists("pnpm") is used.
	checkAvailable func() bool

	// runOutdated overrides `pnpm outdated -g --format json` for testing.
	// When nil, the real command is run. stdout/stderr are returned
	// separately because pnpm's "global bin dir not in PATH" error (which
	// blocks every `-g` command until `pnpm setup` is run) lands on stderr
	// while stdout stays empty.
	runOutdated func(ctx context.Context) (stdout, stderr string, err error)

	// runUpdate overrides `pnpm update -g --latest <names...>` for testing.
	// When nil, the real command is run.
	runUpdate func(ctx context.Context, names []string) (string, error)
}

// NewPnpmProvider creates a new pnpm provider.
func NewPnpmProvider(cfg config.PnpmConfig, logger *logging.Logger) *PnpmProvider {
	return &PnpmProvider{cfg: cfg, logger: logger}
}

func (p *PnpmProvider) Name() string        { return "pnpm" }
func (p *PnpmProvider) DisplayName() string { return "pnpm Global Packages" }
func (p *PnpmProvider) DependsOn() []string { return nil }

// Scan runs `pnpm outdated -g --format json` and returns outdated global
// packages. pnpm exits with code 1 when packages are outdated — we treat
// that as non-error, matching npm's behavior.
func (p *PnpmProvider) Scan(ctx context.Context) ScanResult {
	if !p.isAvailable() {
		return ScanResult{Available: false, Message: "pnpm not found"}
	}

	stdout, stderr, _ := p.doOutdated(ctx)

	// pnpm requires `pnpm setup` to add its global bin dir to PATH before any
	// `-g`/`--global` command works; without it, both outdated and update
	// fail with this error on stderr (stdout stays empty) rather than an
	// empty result.
	if isPnpmGlobalBinNotInPath(stderr) {
		return ScanResult{
			Available: true,
			Message:   "pnpm: global bin directory not in PATH — run `pnpm setup`",
		}
	}

	if stdout == "" || stdout == "{}" || stdout == "null" {
		return ScanResult{Available: true, Outdated: nil}
	}

	items, err := parsePnpmOutdated(stdout)
	if err != nil {
		p.logf("parsing pnpm outdated output: %v", err)
		return ScanResult{Available: true, Error: err}
	}

	return ScanResult{Available: true, Outdated: items}
}

// Update runs `pnpm update -g --latest <pkg>` for each outdated package.
func (p *PnpmProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	if len(items) == 0 {
		return UpdateResult{}
	}

	start := time.Now()

	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.Name
	}

	// Batch into a single `pnpm update -g --latest a b c`. Concurrent global
	// installs can clobber each other's staging directory, so a single
	// invocation is both faster and safer than parallel processes.
	updated, failed := BatchUpgrade(ctx, names,
		func(ctx context.Context, names []string) (string, error) {
			out, err := p.doUpdate(ctx, names)
			if err != nil {
				p.logf("pnpm update -g (batch) error: %v\n%s", err, out)
			}
			return out, err
		},
		func(ctx context.Context, name string) (string, error) {
			out, err := p.doUpdate(ctx, []string{name})
			if err != nil {
				p.logf("pnpm update -g %s error: %v\n%s", name, err, out)
			}
			return out, err
		},
	)

	return UpdateResult{
		Updated:  updated,
		Failed:   failed,
		Duration: time.Since(start),
	}
}

// isAvailable calls the provider's override if set, otherwise the real
// CommandExists("pnpm") check.
func (p *PnpmProvider) isAvailable() bool {
	if p.checkAvailable != nil {
		return p.checkAvailable()
	}
	return CommandExists("pnpm")
}

// doOutdated calls the provider's override if set, otherwise runs the real
// `pnpm outdated -g --format json` command.
func (p *PnpmProvider) doOutdated(ctx context.Context) (stdout, stderr string, err error) {
	if p.runOutdated != nil {
		return p.runOutdated(ctx)
	}
	return RunCommand(ctx, "pnpm", "outdated", "-g", "--format", "json")
}

// doUpdate calls the provider's override if set, otherwise runs the real
// `pnpm update -g --latest <names...>` command.
func (p *PnpmProvider) doUpdate(ctx context.Context, names []string) (string, error) {
	if p.runUpdate != nil {
		return p.runUpdate(ctx, names)
	}
	args := append([]string{"update", "-g", "--latest"}, names...)
	return RunCommandWithLog(ctx, p.logger, "pnpm", args...)
}

// isPnpmGlobalBinNotInPath reports whether pnpm's outdated/update output
// indicates its global bin directory isn't in PATH yet, in which case every
// `-g`/`--global` command refuses to run. e.g.:
//
//	[ERROR] The configured global bin directory "..." is not in PATH
//	Run "pnpm setup" to update your shell configuration.
func isPnpmGlobalBinNotInPath(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "global bin directory") && strings.Contains(lower, "not in path")
}

// parsePnpmOutdated parses the JSON output of `pnpm outdated -g --format json`.
// The output is a map of package name → version info.
func parsePnpmOutdated(jsonStr string) ([]OutdatedItem, error) {
	var raw map[string]pnpmPackageInfo
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, err
	}

	items := make([]OutdatedItem, 0, len(raw))
	for name, info := range raw {
		items = append(items, OutdatedItem{
			Name:           name,
			CurrentVersion: info.Current,
			LatestVersion:  info.Latest,
		})
	}
	return items, nil
}

func (p *PnpmProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[pnpm] "+format, args...)
	}
}

func init() {
	Register(NewPnpmProvider(config.PnpmConfig{Enabled: true}, nil))
}
