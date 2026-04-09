package provider

import (
	"context"
	"encoding/json"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// npmOutdated represents a single package from `npm outdated -g --json`.
// The JSON is a map of package name → version info.
type npmPackageInfo struct {
	Current  string `json:"current"`
	Wanted   string `json:"wanted"`
	Latest   string `json:"latest"`
	Location string `json:"location"`
}

// NpmProvider implements the npm global packages updater.
type NpmProvider struct {
	cfg    config.NpmConfig
	logger *logging.Logger
}

// NewNpmProvider creates a new npm provider.
func NewNpmProvider(cfg config.NpmConfig, logger *logging.Logger) *NpmProvider {
	return &NpmProvider{cfg: cfg, logger: logger}
}

func (p *NpmProvider) Name() string        { return "npm" }
func (p *NpmProvider) DisplayName() string { return "npm Global Packages" }
func (p *NpmProvider) DependsOn() []string { return nil }

// Scan runs `npm outdated -g --json` and returns outdated packages.
// npm exits with code 1 when packages are outdated — we treat that as non-error.
func (p *NpmProvider) Scan(ctx context.Context) ScanResult {
	if !CommandExists("npm") {
		return ScanResult{Available: false, Message: "npm not found"}
	}

	stdout, _, _ := RunCommand(ctx, "npm", "outdated", "-g", "--json")
	// npm exits 1 when there are outdated packages — ignore the exit code.
	// An empty stdout means everything is up to date.
	if stdout == "" || stdout == "{}" || stdout == "null" {
		return ScanResult{Available: true, Outdated: nil}
	}

	items, err := parseNpmOutdated(stdout)
	if err != nil {
		p.logf("parsing npm outdated output: %v", err)
		return ScanResult{Available: true, Error: err}
	}

	return ScanResult{Available: true, Outdated: items}
}

// Update runs `npm install -g <pkg>@latest` for each outdated package.
func (p *NpmProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	if len(items) == 0 {
		return UpdateResult{}
	}

	start := time.Now()
	var updated, failed []string

	for _, item := range items {
		ReportProgress(ctx, item.Name, PackageStarting)
		out, err := RunCommandWithLog(ctx, p.logger, "npm", "install", "-g", item.Name+"@latest")
		if err != nil {
			p.logf("npm install -g %s@latest error: %v\n%s", item.Name, err, out)
			failed = append(failed, item.Name)
			ReportProgress(ctx, item.Name, PackageFailed)
		} else {
			updated = append(updated, item.Name)
			ReportProgress(ctx, item.Name, PackageUpdated)
		}
	}

	return UpdateResult{
		Updated:  updated,
		Failed:   failed,
		Duration: time.Since(start),
	}
}

// parseNpmOutdated parses the JSON output of `npm outdated -g --json`.
// The output is a map of package name → version info.
func parseNpmOutdated(jsonStr string) ([]OutdatedItem, error) {
	var raw map[string]npmPackageInfo
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

func (p *NpmProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[npm] "+format, args...)
	}
}

func init() {
	Register(NewNpmProvider(config.NpmConfig{Enabled: true}, nil))
}
