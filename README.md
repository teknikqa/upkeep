# upkeep

[![CI](https://github.com/teknikqa/upkeep/actions/workflows/ci.yml/badge.svg)](https://github.com/teknikqa/upkeep/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/teknikqa/upkeep/branch/main/graph/badge.svg)](https://codecov.io/gh/teknikqa/upkeep)
[![Go Version](https://img.shields.io/github/go-mod/go-version/teknikqa/upkeep)](https://go.dev/)
[![License](https://img.shields.io/github/license/teknikqa/upkeep)](LICENSE)
[![Release](https://img.shields.io/github/v/release/teknikqa/upkeep)](https://github.com/teknikqa/upkeep/releases/latest)

A Go CLI tool that keeps your macOS development environment up to date.

![demo](demo.gif)

## Features

- **10 providers**: Homebrew formulae, Homebrew casks, npm, Composer, pip, Rust, VS Code extensions, Oh My Zsh, Vim, Vagrant
- **Scan → Confirm → Execute → Report pipeline** with pterm TUI output
- **Parallel execution** with configurable parallelism and dependency ordering (brew-cask waits for brew)
- **Auth-required cask partitioning**: detects which casks need admin auth via dry-run probe + heuristic fallback; defers them to a separate script
- **Resumability**: JSON state file tracks last-run results; `--retry-failed` re-runs only failed providers
- **Deferred cask script**: `--run-deferred` executes the generated script for auth-required casks
- **YAML config** with per-provider skip lists, auth overrides, and strategy settings — editable via **interactive TUI** or by hand
- **macOS notifications** via `terminal-notifier` (falls back to `osascript`)

## Installation

### From a release

Download the latest archive from the [releases page](https://github.com/teknikqa/upkeep/releases/latest):

```bash
# macOS Apple Silicon (arm64)
curl -sL https://github.com/teknikqa/upkeep/releases/latest/download/upkeep_$(curl -s https://api.github.com/repos/teknikqa/upkeep/releases/latest | grep tag_name | cut -d '"' -f4 | tr -d v)_darwin_arm64.tar.gz | tar xz

# macOS Intel (amd64)
curl -sL https://github.com/teknikqa/upkeep/releases/latest/download/upkeep_$(curl -s https://api.github.com/repos/teknikqa/upkeep/releases/latest | grep tag_name | cut -d '"' -f4 | tr -d v)_darwin_amd64.tar.gz | tar xz

# Move to a directory in your PATH
sudo mv upkeep /usr/local/bin/
```

### From source

Requires Go 1.25+.

```bash
# Install with go install
go install github.com/teknikqa/upkeep@latest

# Or build from source
make build

# Build and install to ~/bin/upkeep
make install
```

## Usage

```bash
# Update all available providers
upkeep

# Scan only — show what would be updated
upkeep --dry-run

# Update without confirmation prompt
upkeep --yes

# Update specific providers
upkeep brew npm

# Re-run only providers that failed last time
upkeep --retry-failed

# Execute deferred auth-required cask updates
upkeep --run-deferred

# Show full subprocess output on console
upkeep --verbose

# List all available providers
upkeep --list

# Use a custom config file
upkeep --config ~/.config/upkeep/config.yaml
```

### Managing Configuration

```bash
# Launch interactive config editor (TUI)
upkeep config edit

# Print current effective configuration as YAML
upkeep config show

# Print config file path
upkeep config path

# Reset configuration to defaults
upkeep config reset
```

## Configuration

Config file location: `~/.config/upkeep/config.yaml` (auto-created with defaults on first run).

Use `upkeep config edit` to modify settings interactively, or edit the file directly:

```yaml
parallelism: 4

providers:
  brew:
    enabled: true
    skip: []           # packages to skip

  brew_cask:
    enabled: true
    greedy: true
    auth_strategy: defer    # defer | skip | force-interactive
    auth_overrides:
      docker: false         # never requires auth
    rebuild_open_with: true

  npm:
    enabled: true
    skip: []

  # ... other providers follow the same pattern

notifications:
  enabled: true
  tool: terminal-notifier   # terminal-notifier | osascript

logging:
  dir: ~/Library/Logs
  level: info
```

## Auth Strategy for Homebrew Casks

Casks that require admin authentication are handled per the `auth_strategy` config:

| Strategy | Behaviour |
|----------|-----------|
| `defer` (default) | Writes `~/.local/state/upkeep/deferred-cask.sh`; sends macOS notification; run later with `--run-deferred` |
| `skip` | Skips auth-required casks entirely |
| `force-interactive` | Runs brew interactively (prompts for password) |

Auth detection priority: **config override** > **dry-run probe** (`NONINTERACTIVE=1 brew upgrade --cask <name> --dry-run`) > **heuristic** (inspects `brew info` for `.pkg`, `installer`, `launchctl`, etc.)

## State File

State is written to `~/.local/state/upkeep/last-run.json` after each run. It records:
- Per-provider status (`success` / `partial` / `failed`)
- Lists of updated / failed / deferred / skipped packages
- Deferred cask script path
- Run timestamp and duration

## Development

```bash
# Run all tests
make test

# Run tests with coverage report
make coverage

# Lint (go vet + golangci-lint)
make lint

# Format code
make fmt

# Run full CI pipeline locally (fmt, lint, test, build)
make ci

# GoReleaser dry-run
make release-dry-run

# Tidy dependencies
make tidy
```
