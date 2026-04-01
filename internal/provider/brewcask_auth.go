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
//     preflight) and uninstall stanzas (launchctl, kext, pkgutil, privileged
//     /Library/ deletes) that typically require admin privileges.

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
type brewCaskInfo struct {
	Artifacts []interface{}            `json:"artifacts"`
	Uninstall []map[string]interface{} `json:"uninstall"`
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
// It examines artifact types (pkg, installer, postflight, preflight) and uninstall
// stanzas (launchctl, kext, pkgutil, /Library/ deletes) that typically require root.
func (p *BrewCaskProvider) heuristicAuthRequired(ctx context.Context, name string) bool {
	stdout, _, err := RunCommand(ctx, "brew", "info", "--cask", "--json=v2", name)
	if err != nil {
		return false
	}

	var info brewCaskInfoV2
	if err := json.Unmarshal([]byte(stdout), &info); err != nil || len(info.Casks) == 0 {
		return false
	}
	cask := info.Casks[0]

	// Check artifacts for pkg, installer, postflight, preflight.
	for _, a := range cask.Artifacts {
		switch v := a.(type) {
		case map[string]interface{}:
			for key := range v {
				switch key {
				case "pkg", "installer", "postflight", "preflight":
					return true
				}
			}
		case string:
			if v == "pkg" || v == "installer" {
				return true
			}
		}
	}

	// Check uninstall stanzas for privileged operations.
	privilegedKeys := []string{"launchctl", "kext", "pkgutil"}
	for _, stanza := range cask.Uninstall {
		for key, val := range stanza {
			// Check well-known privileged keys.
			for _, pk := range privilegedKeys {
				if key == pk {
					return true
				}
			}
			// Check "delete" paths under /Library/.
			if key == "delete" {
				switch v := val.(type) {
				case string:
					if strings.HasPrefix(v, "/Library/") {
						return true
					}
				case []interface{}:
					for _, s := range v {
						if str, ok := s.(string); ok && strings.HasPrefix(str, "/Library/") {
							return true
						}
					}
				}
			}
		}
	}
	return false
}
