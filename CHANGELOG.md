# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- README badges for CI, coverage, Go version, license, and latest release
- `go install` instructions in README
- `.editorconfig` for cross-editor consistency
- Makefile targets: `coverage`, `fmt`, `ci`, `release-dry-run`
- Package comments for `engine` and `cmd` packages
- Codecov integration for test coverage tracking
- `gosec` security linter with targeted exclusions
- This changelog

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
  - VirtualBox extension pack (`virtualbox`)
- **Auth detection**: Config override → dry-run probe → heuristic fallback for cask authentication
- **Deferred cask script**: Auth-required casks written to `~/.local/state/upkeep/deferred-cask.sh` with secure permissions
- **CI**: GitHub Actions for build, test (with race detector), and golangci-lint
- **Release**: GoReleaser for macOS amd64 + arm64 binaries
- **Dependabot**: Weekly updates for Go modules and GitHub Actions

[Unreleased]: https://github.com/teknikqa/upkeep/compare/v0.1.0...HEAD
[0.1.0]: https://github.com/teknikqa/upkeep/releases/tag/v0.1.0
