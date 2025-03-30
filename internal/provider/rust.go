package provider

import (
	"context"
	"strings"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/logging"
)

// RustProvider implements rustup + cargo-install-update updater.
type RustProvider struct {
	cfg    config.RustConfig
	logger *logging.Logger
}

// NewRustProvider creates a new Rust provider.
func NewRustProvider(cfg config.RustConfig, logger *logging.Logger) *RustProvider {
	return &RustProvider{cfg: cfg, logger: logger}
}

func (p *RustProvider) Name() string        { return "rust" }
func (p *RustProvider) DisplayName() string { return "Rust (rustup + cargo)" }
func (p *RustProvider) DependsOn() []string { return nil }

// Scan checks for toolchain updates via `rustup check` and cargo binary updates.
func (p *RustProvider) Scan(ctx context.Context) ScanResult {
	rustupExists := CommandExists("rustup")
	cargoExists := CommandExists("cargo")

	if !rustupExists && !cargoExists {
		return ScanResult{Available: false, Message: "rustup and cargo not found"}
	}

	var items []OutdatedItem

	if rustupExists {
		stdout, _, _ := RunCommand(ctx, "rustup", "check")
		toolchainItems := parseRustupCheck(stdout)
		items = append(items, toolchainItems...)
	}

	if cargoExists && CommandExists("cargo-install-update") {
		stdout, _, _ := RunCommand(ctx, "cargo", "install-update", "-a", "--list")
		cargoItems := parseCargoInstallUpdateList(stdout)
		items = append(items, cargoItems...)
	}

	return ScanResult{Available: true, Outdated: items}
}

// Update runs rustup update and/or cargo install-update -a.
func (p *RustProvider) Update(ctx context.Context, items []OutdatedItem) UpdateResult {
	start := time.Now()
	var updated, failed []string

	if p.cfg.Rustup && CommandExists("rustup") {
		out, err := RunCommandWithLog(ctx, p.logger, "rustup", "update")
		if err != nil {
			p.logf("rustup update error: %v\n%s", err, out)
			failed = append(failed, "rustup-toolchains")
		} else {
			updated = append(updated, "rustup-toolchains")
		}
	}

	if p.cfg.CargoInstallUpdate && CommandExists("cargo") && CommandExists("cargo-install-update") {
		out, err := RunCommandWithLog(ctx, p.logger, "cargo", "install-update", "-a")
		if err != nil {
			p.logf("cargo install-update error: %v\n%s", err, out)
			failed = append(failed, "cargo-binaries")
		} else {
			updated = append(updated, "cargo-binaries")
		}
	}

	return UpdateResult{
		Updated:  updated,
		Failed:   failed,
		Duration: time.Since(start),
	}
}

// parseRustupCheck parses `rustup check` output for outdated toolchains.
// Example line: "stable-aarch64-apple-darwin - Update available : 1.69.0 -> 1.71.0"
func parseRustupCheck(output string) []OutdatedItem {
	var items []OutdatedItem
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.Contains(line, "Update available") {
			continue
		}
		// Extract toolchain name (part before " - ").
		parts := strings.SplitN(line, " - ", 2)
		name := strings.TrimSpace(parts[0])
		// Extract versions from "Update available : X -> Y".
		var current, latest string
		if len(parts) > 1 {
			vparts := strings.SplitN(parts[1], ":", 2)
			if len(vparts) > 1 {
				versions := strings.Split(strings.TrimSpace(vparts[1]), " -> ")
				if len(versions) == 2 {
					current = strings.TrimSpace(versions[0])
					latest = strings.TrimSpace(versions[1])
				}
			}
		}
		items = append(items, OutdatedItem{
			Name:           name,
			CurrentVersion: current,
			LatestVersion:  latest,
		})
	}
	return items
}

// parseCargoInstallUpdateList parses `cargo install-update -a --list` output.
// Lines look like: "    ripgrep              0.9.0        0.9.1           Yes"
func parseCargoInstallUpdateList(output string) []OutdatedItem {
	var items []OutdatedItem
	inTable := false
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		// Header line signals start of table.
		if strings.HasPrefix(line, "Package") && strings.Contains(line, "Installed") {
			inTable = true
			continue
		}
		if !inTable || line == "" || strings.HasPrefix(line, "---") {
			continue
		}
		fields := strings.Fields(line)
		// Fields: [name, installed, latest, needs-update]
		if len(fields) < 4 {
			continue
		}
		if strings.EqualFold(fields[3], "Yes") {
			items = append(items, OutdatedItem{
				Name:           fields[0],
				CurrentVersion: fields[1],
				LatestVersion:  fields[2],
			})
		}
	}
	return items
}

func (p *RustProvider) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Warn("[rust] "+format, args...)
	}
}

func init() {
	Register(NewRustProvider(config.RustConfig{
		Enabled:            true,
		Rustup:             true,
		CargoInstallUpdate: true,
	}, nil))
}
