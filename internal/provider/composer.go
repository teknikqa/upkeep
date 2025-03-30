package provider

import (
	"context"
	"encoding/json"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// composerOutdatedOutput matches `composer global outdated --direct --format=json`.
type composerOutdatedOutput struct {
	Installed []composerPackage `json:"installed"`
}

type composerPackage struct {
	Name         string `json:"name"`
	Version      string `json:"version"`
	Latest       string `json:"latest"`
	LatestStatus string `json:"latest-status"`
	Description  string `json:"description"`
}

// ComposerProvider implements the Composer global packages updater.
type ComposerProvider struct {
	cfg    config.ComposerConfig
	logger *logging.Logger
}

// NewComposerProvider creates a new Composer provider.
func NewComposerProvider(cfg config.ComposerConfig, logger *logging.Logger) *ComposerProvider {
	return &ComposerProvider{cfg: cfg, logger: logger}
}

func (p *ComposerProvider) Name() string        { return "composer" }
func (p *ComposerProvider) DisplayName() string { return "Composer Global Packages" }
func (p *ComposerProvider) DependsOn() []string { return nil }

// Scan runs `composer global outdated --direct --format=json` and returns outdated packages.
func (p *ComposerProvider) Scan(ctx context.Context) ScanResult {
	if !CommandExists("composer") {
		return ScanResult{Available: false, Message: "composer not found"}
	}

	stdout, _, err := RunCommand(ctx, "composer", "global", "outdated", "--direct", "--format=json")
	if err != nil {
		// composer may exit non-zero when packages are outdated — check if we got output.
		if stdout == "" {
			return ScanResult{Available: true, Error: err, Message: "composer global outdated failed"}
		}
	}

	if stdout == "" {
		return ScanResult{Available: true, Outdated: nil}
	}

	items, parseErr := parseComposerOutdated(stdout)
	if parseErr != nil {
		p.logf("parsing composer outdated output: %v", parseErr)
		return ScanResult{Available: true, Error: parseErr}
	}

	return ScanResult{Available: true, Outdated: items}
}

// Update runs `composer global update` to update all global packages.
func (p *ComposerProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	if len(items) == 0 {
		return UpdateResult{}
	}

	start := time.Now()
	names := make([]string, 0, len(items))
	for _, item := range items {
		names = append(names, item.Name)
	}

	out, err := RunCommandWithLog(ctx, p.logger, "composer", "global", "update")
	if err != nil {
		p.logf("composer global update error: %v\n%s", err, out)
		return UpdateResult{
			Failed:   names,
			Duration: time.Since(start),
		}
	}

	return UpdateResult{
		Updated:  names,
		Duration: time.Since(start),
	}
}

// parseComposerOutdated parses the JSON output of `composer global outdated --format=json`.
func parseComposerOutdated(jsonStr string) ([]OutdatedItem, error) {
	var output composerOutdatedOutput
	if err := json.Unmarshal([]byte(jsonStr), &output); err != nil {
		return nil, err
	}

	items := make([]OutdatedItem, 0, len(output.Installed))
	for _, pkg := range output.Installed {
		items = append(items, OutdatedItem{
			Name:           pkg.Name,
			CurrentVersion: pkg.Version,
			LatestVersion:  pkg.Latest,
		})
	}
	return items, nil
}

func (p *ComposerProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[composer] "+format, args...)
	}
}

func init() {
	Register(NewComposerProvider(config.ComposerConfig{Enabled: true}, nil))
}
