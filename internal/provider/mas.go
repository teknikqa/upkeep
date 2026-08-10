package provider

import (
	"bufio"
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// masOutdatedLine matches a single line of `mas outdated --json`, which
// emits one JSON object per outdated app (newline-delimited, not a JSON
// array). Only the fields we need are declared; the rest of the app's
// Spotlight metadata is ignored.
type masOutdatedLine struct {
	AdamID     uint64 `json:"adamID"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	NewVersion string `json:"newVersion"`
}

// MasProvider implements the Mac App Store (mas) updater.
type MasProvider struct {
	cfg    config.MasConfig
	logger *logging.Logger
}

// NewMasProvider creates a new Mac App Store provider.
func NewMasProvider(cfg config.MasConfig, logger *logging.Logger) *MasProvider {
	return &MasProvider{cfg: cfg, logger: logger}
}

func (p *MasProvider) Name() string        { return "mas" }
func (p *MasProvider) DisplayName() string { return "Mac App Store Apps" }
func (p *MasProvider) DependsOn() []string { return nil }

// Scan runs `mas outdated --json` and returns outdated App Store apps.
func (p *MasProvider) Scan(ctx context.Context) ScanResult {
	if !CommandExists("mas") {
		return ScanResult{Available: false, Message: "mas not found"}
	}

	stdout, _, err := RunCommand(ctx, "mas", "outdated", "--json")
	if err != nil {
		return ScanResult{Available: true, Error: err, Message: "mas outdated failed"}
	}

	return ScanResult{Available: true, Outdated: parseMasOutdated(stdout)}
}

// Update upgrades the specified apps. mas re-execs itself via sudo for every
// single update (get/install/update all require root — there is no way to
// opt out), so credentials are pre-cached once before running the batch.
func (p *MasProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	if len(items) == 0 {
		return UpdateResult{}
	}

	start := time.Now()

	names := make([]string, len(items))
	appIDByName := make(map[string]string, len(items))
	for i, item := range items {
		names[i] = item.Name
		appIDByName[item.Name] = item.AppID
	}

	// Pre-cache sudo credentials so mas's internal re-exec can reuse the
	// ticket instead of failing to prompt (RunCommandInteractive gives sudo
	// a real stdin to prompt on when one is available).
	if _, err := RunCommandInteractive(ctx, p.logger, "sudo", "-v"); err != nil {
		p.logf("sudo credential cache failed: %v (mas upgrade will likely fail without a cached ticket)", err)
	}

	toAppIDs := func(names []string) []string {
		ids := make([]string, len(names))
		for i, n := range names {
			ids[i] = appIDByName[n]
		}
		return ids
	}

	// mas processes multiple app IDs within a single re-exec'd process, so
	// batching avoids paying the sudo re-exec cost once per app.
	updated, failed := BatchUpgrade(ctx, names,
		func(ctx context.Context, names []string) (string, error) {
			out, err := RunCommandInteractive(ctx, p.logger, "mas", append([]string{"upgrade"}, toAppIDs(names)...)...)
			if err != nil {
				p.logf("mas upgrade (batch) error: %v\n%s", err, out)
			}
			return out, err
		},
		func(ctx context.Context, name string) (string, error) {
			out, err := RunCommandInteractive(ctx, p.logger, "mas", "upgrade", appIDByName[name])
			if err != nil {
				p.logf("mas upgrade %s error: %v\n%s", name, err, out)
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

// parseMasOutdated parses the newline-delimited JSON output of
// `mas outdated --json`. Lines that fail to parse are skipped rather than
// aborting the whole scan, since each line is independent.
func parseMasOutdated(output string) []OutdatedItem {
	var items []OutdatedItem

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		var m masOutdatedLine
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			continue
		}

		items = append(items, OutdatedItem{
			Name:           m.Name,
			CurrentVersion: m.Version,
			LatestVersion:  m.NewVersion,
			AppID:          strconv.FormatUint(m.AdamID, 10),
		})
	}

	return items
}

func (p *MasProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[mas] "+format, args...)
	}
}

func init() {
	Register(NewMasProvider(config.MasConfig{Enabled: true}, nil))
}
