package provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// VSCodeProvider implements extension updates for VS Code and compatible editors.
type VSCodeProvider struct {
	cfg    config.VSCodeConfig
	logger *logging.Logger
}

// NewVSCodeProvider creates a new VS Code provider.
func NewVSCodeProvider(cfg config.VSCodeConfig, logger *logging.Logger) *VSCodeProvider {
	return &VSCodeProvider{cfg: cfg, logger: logger}
}

func (p *VSCodeProvider) Name() string        { return "vscode" }
func (p *VSCodeProvider) DisplayName() string { return "VS Code / Editors" }
func (p *VSCodeProvider) DependsOn() []string { return nil }

// Scan checks which configured editors are installed and counts their extensions.
func (p *VSCodeProvider) Scan(ctx context.Context) ScanResult {
	editors := p.cfg.Editors
	if len(editors) == 0 {
		editors = []string{"code", "cursor", "kiro", "windsurf", "agy"}
	}

	var items []OutdatedItem
	for _, editor := range editors {
		if !CommandExists(editor) {
			continue
		}
		// Count extensions for informational display.
		count := p.countExtensions(ctx, editor)
		items = append(items, OutdatedItem{
			Name:          editor,
			LatestVersion: fmt.Sprintf("%d extensions", count),
		})
	}

	if len(items) == 0 {
		return ScanResult{Available: false, Message: "no editors found"}
	}
	return ScanResult{Available: true, Outdated: items}
}

// countExtensions returns the number of installed extensions for an editor.
func (p *VSCodeProvider) countExtensions(ctx context.Context, editor string) int {
	stdout, _, err := RunCommand(ctx, editor, "--list-extensions")
	if err != nil {
		return 0
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	count := 0
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			count++
		}
	}
	return count
}

// Update runs `<editor> --update-extensions` for each available editor with timeout.
func (p *VSCodeProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	if len(items) == 0 {
		return UpdateResult{}
	}

	start := time.Now()
	var updated, failed []string

	timeoutSecs := p.cfg.Timeout
	if timeoutSecs <= 0 {
		timeoutSecs = 300
	}

	for _, item := range items {
		if !CommandExists(item.Name) {
			continue
		}
		editorCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSecs)*time.Second)
		out, err := RunCommandWithLog(editorCtx, p.logger, item.Name, "--update-extensions")
		cancel()
		if err != nil {
			if editorCtx.Err() == context.DeadlineExceeded {
				p.logf("%s --update-extensions timed out after %ds", item.Name, timeoutSecs)
			} else {
				p.logf("%s --update-extensions error: %v\n%s", item.Name, err, out)
			}
			failed = append(failed, item.Name)
		} else {
			updated = append(updated, item.Name)
		}
	}

	return UpdateResult{
		Updated:  updated,
		Failed:   failed,
		Duration: time.Since(start),
	}
}

func (p *VSCodeProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[vscode] "+format, args...)
	}
}

func init() {
	Register(NewVSCodeProvider(config.VSCodeConfig{
		Enabled: true,
		Editors: []string{"code", "cursor", "kiro", "windsurf", "agy"},
		Timeout: 300,
	}, nil))
}
