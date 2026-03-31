package provider

import (
	"context"
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

// Scan checks which configured editors are installed.
// Extensions cannot be checked for outdated status without marketplace APIs,
// so Scan reports zero outdated. Update() discovers editors independently.
func (p *VSCodeProvider) Scan(ctx context.Context) ScanResult {
	editors := p.cfg.Editors
	if len(editors) == 0 {
		editors = []string{"code", "cursor", "kiro", "windsurf", "agy"}
	}

	found := false
	for _, editor := range editors {
		if CommandExists(editor) {
			found = true
			break
		}
	}

	if !found {
		return ScanResult{Available: false, Message: "no editors found"}
	}
	return ScanResult{Available: true, AlwaysUpdate: true}
}

// Update runs `<editor> --update-extensions` for each available editor with timeout.
// Unlike other providers, this ignores items (Scan reports no outdated) and
// discovers editors directly from config.
func (p *VSCodeProvider) Update(ctx context.Context, _ []OutdatedItem) UpdateResult {
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
		} else {
			updated = append(updated, editor)
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
