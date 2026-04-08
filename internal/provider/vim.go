package provider

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

const pathogenURL = "https://raw.githubusercontent.com/tpope/vim-pathogen/master/autoload/pathogen.vim"

// VimProvider implements Vim plugin management via pathogen + git bundles.
type VimProvider struct {
	cfg    config.VimConfig
	logger *logging.Logger
}

// NewVimProvider creates a new Vim provider.
func NewVimProvider(cfg config.VimConfig, logger *logging.Logger) *VimProvider {
	return &VimProvider{cfg: cfg, logger: logger}
}

func (p *VimProvider) Name() string        { return "vim" }
func (p *VimProvider) DisplayName() string { return "Vim Plugins" }
func (p *VimProvider) DependsOn() []string { return nil }

// Scan checks if an update script or pathogen/bundles exist.
func (p *VimProvider) Scan(ctx context.Context) ScanResult {
	// If external update script is configured and executable, use it.
	if p.cfg.UpdateScript != "" {
		if info, err := os.Stat(p.cfg.UpdateScript); err == nil && !info.IsDir() {
			return ScanResult{
				Available: true,
				Outdated:  []OutdatedItem{{Name: "vim (external script)", LatestVersion: "run"}},
			}
		}
	}

	// Check pathogen dir.
	pathogenExists := false
	if p.cfg.PathogenDir != "" {
		if _, err := os.Stat(p.cfg.PathogenDir); err == nil {
			pathogenExists = true
		}
	}

	// Count git bundles.
	bundleCount := p.countBundles()

	if !pathogenExists && bundleCount == 0 {
		return ScanResult{Available: false, Message: "no vim pathogen or bundles found"}
	}

	var items []OutdatedItem
	if pathogenExists {
		items = append(items, OutdatedItem{Name: "pathogen.vim", LatestVersion: "upstream"})
	}
	if bundleCount > 0 {
		items = append(items, OutdatedItem{
			Name:          fmt.Sprintf("%d bundles", bundleCount),
			LatestVersion: "upstream",
		})
	}

	return ScanResult{Available: true, Outdated: items}
}

// countBundles returns the number of git-managed vim bundles.
func (p *VimProvider) countBundles() int {
	if p.cfg.BundlesDir == "" {
		return 0
	}
	entries, err := os.ReadDir(p.cfg.BundlesDir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		gitDir := filepath.Join(p.cfg.BundlesDir, e.Name(), ".git")
		if _, err := os.Stat(gitDir); err == nil {
			count++
		}
	}
	return count
}

// Update either runs external script or updates pathogen + git bundles.
func (p *VimProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	if len(items) == 0 {
		return UpdateResult{}
	}

	start := time.Now()

	// Delegate to external script if configured.
	if p.cfg.UpdateScript != "" {
		if info, err := os.Stat(p.cfg.UpdateScript); err == nil && !info.IsDir() {
			out, err := RunCommandWithLog(ctx, p.logger, p.cfg.UpdateScript)
			if err != nil {
				p.logf("external vim script error: %v\n%s", err, out)
				return UpdateResult{Failed: []string{"vim"}, Duration: time.Since(start)}
			}
			return UpdateResult{Updated: []string{"vim"}, Duration: time.Since(start)}
		}
	}

	var updated, failed []string

	// Update pathogen.
	if p.cfg.PathogenDir != "" {
		if _, err := os.Stat(p.cfg.PathogenDir); err == nil {
			dest := filepath.Join(p.cfg.PathogenDir, "pathogen.vim")
			if err := downloadFile(ctx, pathogenURL, dest); err != nil {
				p.logf("downloading pathogen: %v", err)
				failed = append(failed, "pathogen.vim")
				ReportProgress(ctx, "pathogen.vim", PackageFailed)
			} else {
				updated = append(updated, "pathogen.vim")
				ReportProgress(ctx, "pathogen.vim", PackageUpdated)
			}
		}
	}

	// Update git bundles.
	if p.cfg.BundlesDir != "" {
		entries, err := os.ReadDir(p.cfg.BundlesDir)
		if err == nil {
			for _, e := range entries {
				if !e.IsDir() {
					continue
				}
				bundleDir := filepath.Join(p.cfg.BundlesDir, e.Name())
				gitDir := filepath.Join(bundleDir, ".git")
				if _, err := os.Stat(gitDir); os.IsNotExist(err) {
					continue
				}
				out, err := RunCommandWithLog(ctx, p.logger, "git", "-C", bundleDir, "pull", "--rebase")
				if err != nil {
					p.logf("git pull %s error: %v\n%s", e.Name(), err, out)
					failed = append(failed, e.Name())
					ReportProgress(ctx, e.Name(), PackageFailed)
				} else {
					updated = append(updated, e.Name())
					ReportProgress(ctx, e.Name(), PackageUpdated)
				}
			}
		}
	}

	return UpdateResult{
		Updated:  updated,
		Failed:   failed,
		Duration: time.Since(start),
	}
}

// downloadFile downloads url to dest using the provided context, creating
// parent directories as needed.
func downloadFile(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request for %s: %w", url, err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("downloading %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return fmt.Errorf("creating dir for %s: %w", dest, err)
	}

	f, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", dest, err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("writing %s: %w", dest, err)
	}
	return nil
}

func (p *VimProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[vim] "+format, args...)
	}
}

func init() {
	Register(NewVimProvider(config.VimConfig{
		Enabled:      true,
		UpdateScript: "",
		PathogenDir:  "",
		BundlesDir:   "",
	}, nil))
}
