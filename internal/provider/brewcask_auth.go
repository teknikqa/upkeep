package provider

// brewcask_auth.go contains the three-tier authentication detection logic for
// Homebrew casks. The three tiers are used in priority order:
//
//  1. Config override (cfg.AuthOverrides): Explicit per-cask override set by the
//     user in the config file. Highest priority — always respected if present.
//
//  2. Dry-run probe (probeAuthRequired): Runs `NONINTERACTIVE=1 brew upgrade
//     --cask <name> --dry-run` and inspects stderr for auth-related keywords.
//     Most reliable when it gives a conclusive answer; may be inconclusive for
//     network errors or Homebrew internal issues.
//
//  3. Heuristic fallback (heuristicAuthRequired): Inspects `brew info --cask
//     --json=v2 <name>` metadata for artifact types (pkg, installer, postflight,
//     preflight) and uninstall stanzas (kext, pkgutil, privileged /Library/
//     deletes) that typically require admin privileges.

import (
	"context"
	"encoding/json"
	"strings"
)

// brewCaskInfoV2 matches the response from `brew info --cask --json=v2 <name>`.
type brewCaskInfoV2 struct {
	Casks []brewCaskInfo `json:"casks"`
}

// brewCaskInfo holds the fields of a single cask relevant to auth detection.
// Homebrew nests uninstall stanzas inside artifacts, e.g.
// {"uninstall": [{"launchctl": [...], "delete": [...]}]}.
type brewCaskInfo struct {
	Artifacts []interface{} `json:"artifacts"`
}

// detectAuthRequired determines whether a cask requires admin auth to update.
// Priority: config override > dry-run probe > heuristic fallback.
func (p *BrewCaskProvider) detectAuthRequired(ctx context.Context, name string) bool {
	// 1. Config override takes highest priority.
	if override, ok := p.cfg.AuthOverrides[name]; ok {
		return override
	}

	// 2. Primary: dry-run probe — most reliable approach.
	if authRequired, ok := p.probeAuthRequired(ctx, name); ok {
		return authRequired
	}

	// 3. Fallback: heuristic via brew info.
	return p.heuristicAuthRequired(ctx, name)
}

// probeAuthRequired uses `NONINTERACTIVE=1 brew upgrade --cask <name> --dry-run` to probe.
// Returns (authRequired, conclusive). When conclusive is false, the caller should
// fall through to the heuristic.
func (p *BrewCaskProvider) probeAuthRequired(ctx context.Context, name string) (bool, bool) {
	env := []string{"NONINTERACTIVE=1"}
	_, stderr, err := RunCommandEnv(ctx, env, "brew", "upgrade", "--cask", name, "--dry-run")
	if err == nil {
		// Dry-run succeeded non-interactively — no auth needed.
		return false, true
	}
	stderrLower := strings.ToLower(stderr)
	// Signs that auth IS required:
	if strings.Contains(stderrLower, "password") ||
		strings.Contains(stderrLower, "sudo") ||
		strings.Contains(stderrLower, "authentication") ||
		strings.Contains(stderrLower, "privileges") {
		return true, true
	}
	// Inconclusive (network error, brew bug, etc.) — fall through to heuristic.
	return false, false
}

// heuristicAuthRequired checks brew cask metadata for signs of privileged install.
func (p *BrewCaskProvider) heuristicAuthRequired(ctx context.Context, name string) bool {
	stdout, _, err := RunCommand(ctx, "brew", "info", "--cask", "--json=v2", name)
	if err != nil {
		return false
	}
	return caskInfoNeedsAuth(stdout)
}

// caskInfoNeedsAuth reports whether `brew info --cask --json=v2` output shows
// artifacts that typically need root: a pkg or installer, pre/postflight
// steps, or an uninstall stanza that touches kexts, pkg receipts or paths
// under /Library/.
func caskInfoNeedsAuth(infoJSON string) bool {
	var info brewCaskInfoV2
	if err := json.Unmarshal([]byte(infoJSON), &info); err != nil || len(info.Casks) == 0 {
		return false
	}
	for _, a := range info.Casks[0].Artifacts {
		switch v := a.(type) {
		case map[string]interface{}:
			for key, val := range v {
				switch key {
				// Current Homebrew names these *_steps; older versions used the bare name.
				case "pkg", "installer", "preflight", "postflight", "preflight_steps", "postflight_steps":
					return true
				case "uninstall":
					if privilegedUninstall(val) {
						return true
					}
				}
			}
		case string:
			if v == "pkg" || v == "installer" {
				return true
			}
		}
	}
	return false
}

// privilegedUninstall reports whether an uninstall artifact's stanzas need root.
func privilegedUninstall(val interface{}) bool {
	stanzas, _ := val.([]interface{})
	for _, s := range stanzas {
		stanza, _ := s.(map[string]interface{})
		for key, v := range stanza {
			switch key {
			// launchctl is left out: Electron apps' per-user ShipIt updaters use
			// it too (VS Code, Slack), and removing those needs no root.
			case "kext", "pkgutil":
				return true
			case "delete":
				paths, ok := v.([]interface{})
				if !ok {
					paths = []interface{}{v}
				}
				for _, path := range paths {
					if str, ok := path.(string); ok && strings.HasPrefix(str, "/Library/") {
						return true
					}
				}
			}
		}
	}
	return false
}
