package provider

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// bunTableBorderRe matches a pure ASCII-table border/separator line, e.g.
// "|---------------------------------------|" or "|----------|---------|".
var bunTableBorderRe = regexp.MustCompile(`^[-|]+$`)

// BunProvider implements the bun global packages updater. bun's `outdated`
// command has no `--json` flag — only a rendered ASCII table on stdout.
type BunProvider struct {
	cfg    config.BunConfig
	logger *logging.Logger

	// checkAvailable overrides the `bun` binary presence check for testing.
	// When nil, the real CommandExists("bun") is used.
	checkAvailable func() bool

	// runOutdated overrides `bun outdated -g` for testing. When nil, the
	// real command is run. stdout/stderr are returned separately because
	// bun's "missing lockfile" (zero global packages installed yet) error
	// lands on stderr while stdout only has the version banner.
	runOutdated func(ctx context.Context) (stdout, stderr string, err error)

	// runUpdate overrides `bun update -g --latest <names...>` for testing.
	// When nil, the real command is run.
	runUpdate func(ctx context.Context, names []string) (string, error)
}

// NewBunProvider creates a new bun provider.
func NewBunProvider(cfg config.BunConfig, logger *logging.Logger) *BunProvider {
	return &BunProvider{cfg: cfg, logger: logger}
}

func (p *BunProvider) Name() string        { return "bun" }
func (p *BunProvider) DisplayName() string { return "Bun Global Packages" }
func (p *BunProvider) DependsOn() []string { return nil }

// Scan runs `bun outdated -g` and parses its table output for outdated
// global packages.
func (p *BunProvider) Scan(ctx context.Context) ScanResult {
	if !p.isAvailable() {
		return ScanResult{Available: false, Message: "bun not found"}
	}

	stdout, stderr, _ := p.doOutdated(ctx)

	// bun exits 1 with this stderr message when no global packages are
	// installed yet (no global lockfile) — that's zero outdated, not a
	// failure.
	if isBunNoGlobalLockfile(stderr) {
		return ScanResult{Available: true, Outdated: nil}
	}

	return ScanResult{Available: true, Outdated: parseBunOutdatedTable(stdout)}
}

// Update runs `bun update -g --latest <pkg>` for each outdated package.
func (p *BunProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	if len(items) == 0 {
		return UpdateResult{}
	}

	start := time.Now()

	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.Name
	}

	// Batch into a single `bun update -g --latest a b c`. Concurrent global
	// installs can clobber each other's staging directory, so a single
	// invocation is both faster and safer than parallel processes.
	updated, failed := BatchUpgrade(ctx, names,
		func(ctx context.Context, names []string) (string, error) {
			out, err := p.doUpdate(ctx, names)
			if err != nil {
				p.logf("bun update -g (batch) error: %v\n%s", err, out)
			}
			return out, err
		},
		func(ctx context.Context, name string) (string, error) {
			out, err := p.doUpdate(ctx, []string{name})
			if err != nil {
				p.logf("bun update -g %s error: %v\n%s", name, err, out)
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
// CommandExists("bun") check.
func (p *BunProvider) isAvailable() bool {
	if p.checkAvailable != nil {
		return p.checkAvailable()
	}
	return CommandExists("bun")
}

// doOutdated calls the provider's override if set, otherwise runs the real
// `bun outdated -g` command.
func (p *BunProvider) doOutdated(ctx context.Context) (stdout, stderr string, err error) {
	if p.runOutdated != nil {
		return p.runOutdated(ctx)
	}
	return RunCommand(ctx, "bun", "outdated", "-g")
}

// doUpdate calls the provider's override if set, otherwise runs the real
// `bun update -g --latest <names...>` command.
func (p *BunProvider) doUpdate(ctx context.Context, names []string) (string, error) {
	if p.runUpdate != nil {
		return p.runUpdate(ctx, names)
	}
	args := append([]string{"update", "-g", "--latest"}, names...)
	return RunCommandWithLog(ctx, p.logger, "bun", args...)
}

// isBunNoGlobalLockfile reports whether bun's outdated output indicates no
// global packages are installed yet (no global lockfile exists), in which
// case `bun outdated -g` exits 1 with this stderr message instead of
// printing an empty table. e.g.:
//
//	error: missing lockfile, nothing outdated
//	note: run 'bun install' first
func isBunNoGlobalLockfile(output string) bool {
	lower := strings.ToLower(output)
	return strings.Contains(lower, "missing lockfile") && strings.Contains(lower, "nothing outdated")
}

// parseBunOutdatedTable parses the ASCII table printed by `bun outdated -g`,
// e.g.:
//
//	bun outdated v1.4.0 (1381054db)
//	|---------------------------------------|
//	| Package  | Current | Update  | Latest |
//	|----------|---------|---------|--------|
//	| cowsay   | 1.5.0   | 1.5.0   | 1.6.0  |
//	|----------|---------|---------|--------|
//	| lodash   | 4.17.20 | 4.17.20 | 4.18.1 |
//	|---------------------------------------|
func parseBunOutdatedTable(output string) []OutdatedItem {
	var items []OutdatedItem

	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "|") {
			continue // banner line, e.g. "bun outdated v1.4.0 (...)"
		}
		if bunTableBorderRe.MatchString(trimmed) {
			continue // pure border/separator line
		}
		if strings.Contains(trimmed, "Package") && strings.Contains(trimmed, "Latest") {
			continue // header row
		}

		fields := strings.Split(trimmed, "|")
		// Splitting a "|"-delimited line that starts and ends with "|"
		// yields empty leading/trailing elements — drop them.
		if len(fields) > 0 && strings.TrimSpace(fields[0]) == "" {
			fields = fields[1:]
		}
		if len(fields) > 0 && strings.TrimSpace(fields[len(fields)-1]) == "" {
			fields = fields[:len(fields)-1]
		}
		// Columns: Package | Current | Update | Latest.
		if len(fields) < 4 {
			continue
		}

		items = append(items, OutdatedItem{
			Name:           strings.TrimSpace(fields[0]),
			CurrentVersion: strings.TrimSpace(fields[1]),
			LatestVersion:  strings.TrimSpace(fields[3]),
		})
	}

	return items
}

func (p *BunProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[bun] "+format, args...)
	}
}

func init() {
	Register(NewBunProvider(config.BunConfig{Enabled: true}, nil))
}
