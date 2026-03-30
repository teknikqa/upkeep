# upkeep

A Go CLI tool that keeps your macOS development environment up to date.

## Features

- **11 providers**: Homebrew formulae, Homebrew casks, npm, Composer, pip, Rust, VS Code extensions, Oh My Zsh, Vim, Vagrant, VirtualBox
- **Scan → Confirm → Execute → Report pipeline** with pterm TUI output
- **Parallel execution** with configurable parallelism and dependency ordering (brew-cask waits for brew)
- **Auth-required cask partitioning**: detects which casks need admin auth via dry-run probe + heuristic fallback; defers them to a separate script
- **Resumability**: JSON state file tracks last-run results; `--retry-failed` re-runs only failed providers
- **Deferred cask script**: `--run-deferred` executes the generated script for auth-required casks
- **YAML config** with per-provider skip lists, auth overrides, and strategy settings
- **macOS notifications** via `terminal-notifier` (falls back to `osascript`)

## Installation

```bash
# Build and install to ~/bin/upkeep
make install

# Or just build locally
make build
```

Requires Go 1.24+.

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

## Configuration

Config file location: `~/.config/upkeep/config.yaml` (auto-created with defaults on first run).

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
      virtualbox: true      # always requires auth
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

# Lint (go vet + staticcheck)
make lint

# Tidy dependencies
make tidy
```
