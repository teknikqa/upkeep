package provider

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// brewOutdatedV2 matches the structure of `brew outdated --json=v2`.
type brewOutdatedV2 struct {
	Formulae []brewFormula `json:"formulae"`
}

type brewFormula struct {
	Name              string   `json:"name"`
	InstalledVersions []string `json:"installed_versions"`
	CurrentVersion    string   `json:"current_version"`
}

// BrewProvider implements the Homebrew formulae updater.
type BrewProvider struct {
	cfg    config.BrewConfig
	logger *logging.Logger
}

// NewBrewProvider creates a new Homebrew formulae provider.
func NewBrewProvider(cfg config.BrewConfig, logger *logging.Logger) *BrewProvider {
	return &BrewProvider{cfg: cfg, logger: logger}
}

func (p *BrewProvider) Name() string        { return "brew" }
func (p *BrewProvider) DisplayName() string { return "Homebrew Formulae" }
func (p *BrewProvider) DependsOn() []string { return nil }

// Scan runs `brew update` and `brew outdated --json=v2`, returning outdated formulae.
func (p *BrewProvider) Scan(ctx context.Context) ScanResult {
	if !CommandExists("brew") {
		return ScanResult{Available: false, Message: "brew not found"}
	}

	// Run brew update quietly to refresh the package index.
	if _, _, err := RunCommand(ctx, "brew", "update", "--quiet"); err != nil {
		p.logf("brew update error (non-fatal): %v", err)
	}

	stdout, _, err := RunCommand(ctx, "brew", "outdated", "--json=v2")
	if err != nil {
		return ScanResult{
			Available: true,
			Error:     err,
			Message:   "brew outdated failed",
		}
	}

	items, parseErr := parseBrewOutdated(stdout)
	if parseErr != nil {
		// Fall back to empty list; log the parse error.
		p.logf("parsing brew outdated output: %v", parseErr)
		return ScanResult{Available: true, Error: parseErr}
	}

	return ScanResult{
		Available: true,
		Outdated:  items,
	}
}

// Update upgrades the specified formulae and runs post-hooks.
func (p *BrewProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	if len(items) == 0 {
		return UpdateResult{}
	}

	start := time.Now()
	var updated, failed []string

	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}

	args := append([]string{"upgrade", "--quiet"}, names...)
	output, err := RunCommandWithLog(ctx, p.logger, "brew", args...)
	if err != nil {
		p.logf("brew upgrade error: %v\n%s", err, output)
		failed = names
	} else {
		updated = names
	}

	// Run post-hooks from config.
	for _, hook := range p.cfg.PostHooks {
		parts := strings.Fields(hook)
		if len(parts) == 0 {
			continue
		}
		if hookOut, hookErr := RunCommandWithLog(ctx, p.logger, parts[0], parts[1:]...); hookErr != nil {
			p.logf("post-hook %q error: %v\n%s", hook, hookErr, hookOut)
		}
	}

	return UpdateResult{
		Updated:  updated,
		Failed:   failed,
		Duration: time.Since(start),
	}
}

// parseBrewOutdated parses the JSON output of `brew outdated --json=v2`.
func parseBrewOutdated(jsonStr string) ([]OutdatedItem, error) {
	var result brewOutdatedV2
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, err
	}

	items := make([]OutdatedItem, 0, len(result.Formulae))
	for _, f := range result.Formulae {
		installed := ""
		if len(f.InstalledVersions) > 0 {
			installed = f.InstalledVersions[0]
		}
		items = append(items, OutdatedItem{
			Name:           f.Name,
			CurrentVersion: installed,
			LatestVersion:  f.CurrentVersion,
		})
	}
	return items, nil
}

func (p *BrewProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[brew] "+format, args...)
	}
}

// init registers the BrewProvider with sensible defaults.
// The actual provider used at runtime is re-created with config in cmd/root.go.
func init() {
	Register(NewBrewProvider(config.BrewConfig{
		Enabled: true,
		PostHooks: []string{
			"brew doctor --quiet",
			"brew autoremove --quiet",
			"brew cleanup --quiet",
		},
	}, nil))
}
