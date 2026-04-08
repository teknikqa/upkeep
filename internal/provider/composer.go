package provider

import (
	"context"
	"regexp"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// composerDryRunLine matches lines like:
//
//   - Installing vendor/package (v1.0.0)
//   - Updating vendor/package (v1.0.0 => v2.0.0)
//   - Locking vendor/package (v1.0.0)
var composerDryRunLine = regexp.MustCompile(
	`^\s+-\s+(Installing|Updating|Locking)\s+(\S+)\s+\((.+)\)`,
)

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

// Scan runs `composer global update --dry-run` and parses the output for
// packages that would be installed or updated.
func (p *ComposerProvider) Scan(ctx context.Context) ScanResult {
	if !CommandExists("composer") {
		return ScanResult{Available: false, Message: "composer not found"}
	}

	stdout, _, err := RunCommand(ctx, "composer", "global", "update", "--dry-run")
	if err != nil {
		return ScanResult{Available: true, Error: err, Message: "composer global update --dry-run failed"}
	}

	items := parseComposerDryRun(stdout)
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
		for _, n := range names {
			ReportProgress(ctx, n, PackageFailed)
		}
		return UpdateResult{
			Failed:   names,
			Duration: time.Since(start),
		}
	}

	for _, n := range names {
		ReportProgress(ctx, n, PackageUpdated)
	}
	return UpdateResult{
		Updated:  names,
		Duration: time.Since(start),
	}
}

// parseComposerDryRun extracts package operations from `composer global update --dry-run`.
// It looks for "Package operations:" and then parses Installing/Updating lines.
// Locking lines are ignored — they reflect lock-file changes, not actual installs.
func parseComposerDryRun(output string) []OutdatedItem {
	lines := strings.Split(output, "\n")

	// Find the "Package operations:" section — only lines after it are actual
	// install/update operations (lines before may be lock-file operations).
	startIdx := 0
	for i, line := range lines {
		if strings.Contains(line, "Package operations:") {
			startIdx = i + 1
			break
		}
	}

	seen := make(map[string]bool)
	var items []OutdatedItem

	for _, line := range lines[startIdx:] {
		m := composerDryRunLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}

		action := m[1]  // Installing | Updating
		name := m[2]    // vendor/package
		verInfo := m[3] // "v1.0.0" or "v1.0.0 => v2.0.0"

		// Skip Locking lines — they appear before "Package operations:" but
		// the guard above should already exclude them. Belt-and-suspenders.
		if action == "Locking" {
			continue
		}

		// Deduplicate in case a package appears in both lock and install sections.
		if seen[name] {
			continue
		}
		seen[name] = true

		item := OutdatedItem{Name: name}
		if action == "Updating" {
			parts := strings.SplitN(verInfo, " => ", 2)
			if len(parts) == 2 {
				item.CurrentVersion = strings.TrimSpace(parts[0])
				item.LatestVersion = strings.TrimSpace(parts[1])
			} else {
				item.LatestVersion = strings.TrimSpace(verInfo)
			}
		} else {
			// Installing — no current version.
			item.LatestVersion = strings.TrimSpace(verInfo)
		}

		items = append(items, item)
	}

	return items
}

func (p *ComposerProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[composer] "+format, args...)
	}
}

func init() {
	Register(NewComposerProvider(config.ComposerConfig{Enabled: true}, nil))
}
