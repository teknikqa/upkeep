package provider

import (
	"bufio"
	"context"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// MacportsProvider implements the MacPorts (port) updater.
type MacportsProvider struct {
	cfg    config.MacportsConfig
	logger *logging.Logger

	// listOutdated overrides the `port -q outdated` query for testing.
	// When set, it also bypasses the CommandExists("port") gate in Scan,
	// since port isn't preinstalled in most CI environments.
	listOutdated func(ctx context.Context) (string, error)
}

// NewMacportsProvider creates a new MacPorts provider.
func NewMacportsProvider(cfg config.MacportsConfig, logger *logging.Logger) *MacportsProvider {
	return &MacportsProvider{cfg: cfg, logger: logger}
}

func (p *MacportsProvider) Name() string        { return "macports" }
func (p *MacportsProvider) DisplayName() string { return "MacPorts" }
func (p *MacportsProvider) DependsOn() []string { return nil }

// Scan runs `port -q outdated` and returns outdated ports.
func (p *MacportsProvider) Scan(ctx context.Context) ScanResult {
	if p.listOutdated == nil && !CommandExists("port") {
		return ScanResult{Available: false, Message: "port not found"}
	}

	stdout, err := p.runOutdated(ctx)
	if err != nil {
		return ScanResult{Available: true, Error: err, Message: "port outdated failed"}
	}

	return ScanResult{Available: true, Outdated: parseMacportsOutdated(stdout)}
}

// runOutdated calls the provider's override if set, otherwise runs the real
// `port -q outdated` command. -q suppresses the header line and the "no
// ports are outdated" message, leaving one line per outdated port (or none).
func (p *MacportsProvider) runOutdated(ctx context.Context) (string, error) {
	if p.listOutdated != nil {
		return p.listOutdated(ctx)
	}
	stdout, _, err := RunCommand(ctx, "port", "-q", "outdated")
	return stdout, err
}

// Update upgrades the specified ports. MacPorts installs into /opt/local and
// requires root for every upgrade (there is no way to opt out), so sudo
// credentials are pre-cached once before running the batch — mirrors the
// mas provider's approach.
func (p *MacportsProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	if len(items) == 0 {
		return UpdateResult{}
	}

	start := time.Now()

	names := make([]string, len(items))
	for i, item := range items {
		names[i] = item.Name
	}

	if _, err := RunCommandInteractive(ctx, p.logger, "sudo", "-v"); err != nil {
		p.logf("sudo credential cache failed: %v (port upgrade will likely fail without a cached ticket)", err)
	}

	updated, failed := BatchUpgrade(ctx, names,
		func(ctx context.Context, names []string) (string, error) {
			out, err := RunCommandInteractive(ctx, p.logger, "sudo", append([]string{"port", "upgrade"}, names...)...)
			if err != nil {
				p.logf("port upgrade (batch) error: %v\n%s", err, out)
			}
			return out, err
		},
		func(ctx context.Context, name string) (string, error) {
			out, err := RunCommandInteractive(ctx, p.logger, "sudo", "port", "upgrade", name)
			if err != nil {
				p.logf("port upgrade %s error: %v\n%s", name, err, out)
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

// parseMacportsOutdated parses the output of `port -q outdated`, one line
// per outdated port in the form "<name> <installed> < <latest>".
func parseMacportsOutdated(output string) []OutdatedItem {
	var items []OutdatedItem

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) != 4 || fields[2] != "<" {
			continue
		}
		items = append(items, OutdatedItem{
			Name:           fields[0],
			CurrentVersion: fields[1],
			LatestVersion:  fields[3],
		})
	}

	return items
}

func (p *MacportsProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[macports] "+format, args...)
	}
}

func init() {
	Register(NewMacportsProvider(config.MacportsConfig{Enabled: true}, nil))
}
