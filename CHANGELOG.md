# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.8.0] - 2026-06-12

### Changed

- **Faster updates via batching.** Homebrew formulae, Homebrew casks (non-auth), npm global packages, and pip packages are now upgraded in a single batched command per provider instead of one-by-one. This is both faster (one process startup, with the package manager fetching downloads concurrently internally) and, for Homebrew specifically, the only way to parallelize — concurrent `brew` processes contend on Homebrew's global lock and cannot run simultaneously. Batching is also safer than firing concurrent `npm install -g` / `pip install` processes, which can corrupt shared install directories.
- Failure isolation is retained: if a batched command reports an error, each package is retried individually so the updated/failed split stays exact (already-upgraded packages become fast no-ops). This restores the speed of the pre-0.7.0 batch behavior without its all-or-nothing failure reporting.
- The update footer now shows batched packages as a group rather than naming the single in-progress package — the per-package progress added in 0.7.0 is coarser for these providers as a result.
- Code editor extension updates now run concurrently across editors (each editor is an independent binary with no shared lock) instead of sequentially.

## [0.7.0] - 2026-04-10

### Changed

- **BREAKING**: Removed VirtualBox provider — VirtualBox is installable via Homebrew cask, making the dedicated provider redundant. Remove `virtualbox:` from your config if present.
- Homebrew formulae and Composer packages are now upgraded one-by-one instead of in a single batch command. The update footer now shows the specific package being updated (e.g. `⏳ Updating: Homebrew Formulae → cryptography`) instead of listing all packages at once.
- Per-package upgrades isolate failures — one formula failing no longer marks all as failed.

## [0.6.0] - 2026-04-09

### Added

- Update footer now shows the current package name being processed.

### Changed

- Providers are now sorted alphabetically by display name.

### Fixed

- Show package names in the live update table during execution.
- Pre-cache sudo credentials for force-interactive cask updates so the prompt doesn't interrupt mid-run.
- Skip pip3 upgrades in PEP 668 externally-managed environments.

## [0.5.0] - 2026-04-08

### Added

- Live per-package progress in the update table.

## [0.4.0] - 2026-04-07

### Added

- Live-updating scan table with per-provider progress.
- Providers with no outdated packages now show an "up to date" status, and all provider sub-groups are shown at all times in the scan table.
- Custom error types with improved context-cancellation handling.

### Changed

- Minimum Go version raised to 1.25.

## [0.3.1] - 2026-04-01

### Fixed

- Editor extensions: skip pre-release versions when checking for outdated extensions.

## [0.3.0] - 2026-04-01

### Added

- **Unified live table**: the live update table and final report are merged into a single table, and the progress bar is replaced by a live-updating scan table during updates.
- Editor extension outdated-version detection via marketplace APIs, with outdated extensions sub-grouped by editor in the scan summary.

### Changed

- Renamed the `vscode` provider to `editor`; its display name is now "Code Editor Extensions".

### Fixed

- Composer: detect pending installs/updates via a dry-run update, and handle bare `[]` JSON when no global packages are installed.
- Editors are no longer falsely reported as outdated packages.
- UI: wrap long package lists into continuation rows instead of overflowing, strip trailing padding from table rows, and show all outdated packages in the scan summary.

## [0.2.0] - 2026-03-31

### Added

- **Interactive config editor**: `upkeep config edit` launches a menu-driven TUI for editing all configuration settings — providers, parallelism, notifications, logging — with type-appropriate editors (toggles, number inputs, enum selects, list management)
- **Config subcommands**: `upkeep config show` (print effective config as YAML), `upkeep config path` (print config file location), `upkeep config reset` (restore defaults with confirmation)
- `config.Save()` function for validated config persistence with automatic directory creation
- Exported `config.Validate()` and `config.Defaults()` for reuse across packages
- Notification tool validation (`terminal-notifier` or `osascript`) in config validation

### Changed

- README updated with config management documentation

## [0.1.0] - 2026-03-30

### Added

- **CLI**: Cobra-based command with flags for `--dry-run`, `--yes`, `--verbose`, `--list`, `--retry-failed`, `--run-deferred`, `--force-interactive`, and `--config`
- **Config**: YAML configuration at `~/.config/upkeep/config.yaml` with per-provider settings, skip lists, auth overrides, and validation
- **State**: JSON state file at `~/.local/state/upkeep/last-run.json` with file locking for resumability
- **Logging**: Date-rotated file logger with configurable level filtering
- **Engine**: Scan → Confirm → Execute → Report pipeline with parallel execution and dependency ordering
- **TUI**: pterm-based scan summary tables, progress bars, and confirmation prompts
- **Notifications**: macOS Notification Center via `terminal-notifier` (falls back to `osascript`)
- **11 providers**:
  - Homebrew formulae (`brew`)
  - Homebrew casks (`brew-cask`) with auth-required cask partitioning and deferred script generation
  - npm global packages (`npm`)
  - Composer global packages (`composer`)
  - pip packages (`pip`)
  - Rust toolchain (`rust`)
  - VS Code / Cursor / Windsurf / Kiro / Agy extensions (`vscode`)
  - Oh My Zsh (`omz`)
  - Vim plugins via vim-plug (`vim`)
  - Vagrant boxes (`vagrant`)
- **Auth detection**: Config override → dry-run probe → heuristic fallback for cask authentication
- **Deferred cask script**: Auth-required casks written to `~/.local/state/upkeep/deferred-cask.sh` with secure permissions
- **CI**: GitHub Actions for build, test (with race detector), and golangci-lint
- **Release**: GoReleaser for macOS amd64 + arm64 binaries
- **Dependabot**: Weekly updates for Go modules and GitHub Actions

[Unreleased]: https://github.com/teknikqa/upkeep/compare/v0.8.0...HEAD
[0.8.0]: https://github.com/teknikqa/upkeep/compare/v0.7.0...v0.8.0
[0.7.0]: https://github.com/teknikqa/upkeep/compare/v0.6.0...v0.7.0
[0.6.0]: https://github.com/teknikqa/upkeep/compare/v0.5.0...v0.6.0
[0.5.0]: https://github.com/teknikqa/upkeep/compare/v0.4.0...v0.5.0
[0.4.0]: https://github.com/teknikqa/upkeep/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/teknikqa/upkeep/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/teknikqa/upkeep/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/teknikqa/upkeep/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/teknikqa/upkeep/releases/tag/v0.1.0
