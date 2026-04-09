package provider

import (
	"context"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
	"github.com/teknikqa/upkeep/internal/marketplace"
)

// EditorProvider implements extension updates for VS Code and compatible editors.
type EditorProvider struct {
	cfg    config.EditorConfig
	logger *logging.Logger
}

// NewEditorProvider creates a new code editor provider.
func NewEditorProvider(cfg config.EditorConfig, logger *logging.Logger) *EditorProvider {
	return &EditorProvider{cfg: cfg, logger: logger}
}

func (p *EditorProvider) Name() string        { return "editor" }
func (p *EditorProvider) DisplayName() string { return "Code Editor Extensions" }
func (p *EditorProvider) DependsOn() []string { return nil }

// SubGroups returns the configured editor names so the UI can show sub-group
// rows before scanning completes.
func (p *EditorProvider) SubGroups() []string {
	editors := p.cfg.Editors
	if len(editors) == 0 {
		editors = []string{"code", "cursor", "kiro", "windsurf", "agy"}
	}
	return editors
}

// Scan checks which configured editors are installed and queries marketplace APIs
// to detect which extensions are actually outdated.
// If marketplace queries fail, Scan degrades gracefully: AlwaysUpdate ensures
// Update() still runs --update-extensions even with no detected outdated items.
func (p *EditorProvider) Scan(ctx context.Context) ScanResult {
	editors := p.cfg.Editors
	if len(editors) == 0 {
		editors = []string{"code", "cursor", "kiro", "windsurf", "agy"}
	}

	found := false
	var allOutdated []OutdatedItem
	groups := make(map[string][]string)

	// Create marketplace clients (reused across editors with the same marketplace).
	httpClient := marketplace.DefaultHTTPClient()
	clients := map[marketplace.MarketplaceType]marketplace.Client{
		marketplace.VSMarketplace: marketplace.NewVSMarketplaceClient(httpClient),
		marketplace.OpenVSX:       marketplace.NewOpenVSXClient(httpClient),
	}

	for _, editor := range editors {
		if !CommandExists(editor) {
			continue
		}
		found = true

		// List installed extensions with versions.
		stdout, _, err := RunCommand(ctx, editor, "--list-extensions", "--show-versions")
		if err != nil {
			p.logf("%s --list-extensions error: %v", editor, err)
			continue
		}

		installed := parseExtensionList(stdout)
		if len(installed) == 0 {
			continue
		}

		// Determine marketplace for this editor (config override takes precedence).
		mpType := marketplace.EditorMarketplace(editor)
		if override, ok := p.cfg.Marketplace[editor]; ok {
			mpType = marketplace.MarketplaceType(override)
		}

		client, ok := clients[mpType]
		if !ok {
			p.logf("unknown marketplace type %q for editor %s", mpType, editor)
			continue
		}

		// Collect extension IDs for batch query.
		ids := make([]string, len(installed))
		for i, ext := range installed {
			ids[i] = ext.ID
		}

		// Query marketplace for latest versions.
		latest, err := client.GetLatestVersions(ctx, ids)
		if err != nil {
			p.logf("%s marketplace query error: %v (falling back to update-all)", editor, err)
			continue
		}

		// Compare installed vs latest and accumulate outdated items.
		editorOutdated := compareVersions(installed, latest)
		allOutdated = append(allOutdated, editorOutdated...)
		if len(editorOutdated) > 0 {
			names := make([]string, len(editorOutdated))
			for i, item := range editorOutdated {
				names[i] = item.Name
			}
			groups[editor] = names
		}
	}

	if !found {
		return ScanResult{Available: false, Message: "no editors found"}
	}

	// AlwaysUpdate: true ensures Update() runs --update-extensions even if
	// marketplace queries fail and Outdated is empty.
	return ScanResult{
		Available:    true,
		Outdated:     allOutdated,
		AlwaysUpdate: true,
		Groups:       groups,
	}
}

// Update runs `<editor> --update-extensions` for each available editor with timeout.
// Unlike other providers, this ignores items (Scan reports no outdated) and
// discovers editors directly from config.
func (p *EditorProvider) Update(ctx context.Context, _ []OutdatedItem) UpdateResult {
	editors := p.cfg.Editors
	if len(editors) == 0 {
		editors = []string{"code", "cursor", "kiro", "windsurf", "agy"}
	}

	start := time.Now()
	var updated, failed []string

	timeoutSecs := p.cfg.Timeout
	if timeoutSecs <= 0 {
		timeoutSecs = 300
	}

	for _, editor := range editors {
		if !CommandExists(editor) {
			continue
		}
		ReportProgress(ctx, editor, PackageStarting)
		editorCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
		out, err := RunCommandWithLog(editorCtx, p.logger, editor, "--update-extensions")
		cancel()
		if err != nil {
			if editorCtx.Err() == context.DeadlineExceeded {
				p.logf("%s --update-extensions timed out after %ds", editor, timeoutSecs)
			} else {
				p.logf("%s --update-extensions error: %v\n%s", editor, err, out)
			}
			failed = append(failed, editor)
			ReportProgress(ctx, editor, PackageFailed)
		} else {
			updated = append(updated, editor)
			ReportProgress(ctx, editor, PackageUpdated)
		}
	}

	return UpdateResult{
		Updated:  updated,
		Failed:   failed,
		Duration: time.Since(start),
	}
}

func (p *EditorProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[editor] "+format, args...)
	}
}

// parseExtensionList parses the output of `<editor> --list-extensions --show-versions`.
// Each line is expected to be in "publisher.extension@version" format.
// Lines that don't match are silently skipped.
func parseExtensionList(output string) []marketplace.Extension {
	var extensions []marketplace.Extension
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		atIdx := strings.LastIndex(line, "@")
		if atIdx <= 0 {
			continue
		}
		extensions = append(extensions, marketplace.Extension{
			ID:      strings.ToLower(line[:atIdx]),
			Version: line[atIdx+1:],
		})
	}
	return extensions
}

// compareVersions takes installed extensions and marketplace latest versions,
// returns OutdatedItems for extensions where local version differs from marketplace.
// Pre-release handling:
//   - If lv.PreRelease is true (no stable version exists at all), the extension is skipped.
//   - If the installed version matches lv.LatestPreReleaseVersion, the user is on the
//     pre-release track intentionally — skip it too.
func compareVersions(installed []marketplace.Extension, latest map[string]marketplace.LatestVersion) []OutdatedItem {
	var outdated []OutdatedItem
	for _, ext := range installed {
		lv, ok := latest[ext.ID]
		if !ok || !lv.Found {
			continue // not in marketplace (private/local), skip
		}
		// No stable version exists — nothing to upgrade to.
		if lv.PreRelease {
			continue
		}
		// Installed version is the latest pre-release — user is on the pre-release track.
		if lv.LatestPreReleaseVersion != "" && ext.Version == lv.LatestPreReleaseVersion {
			continue
		}
		if lv.Version != ext.Version {
			outdated = append(outdated, OutdatedItem{
				Name:           ext.ID,
				CurrentVersion: ext.Version,
				LatestVersion:  lv.Version,
			})
		}
	}
	return outdated
}

func init() {
	Register(NewEditorProvider(config.EditorConfig{
		Enabled: true,
		Editors: []string{"code", "cursor", "kiro", "windsurf", "agy"},
		Timeout: 300,
	}, nil))
}
