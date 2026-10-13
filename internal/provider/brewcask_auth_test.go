package provider_test

import (
	"testing"

	"github.com/teknikqa/upkeep/internal/provider"
)

func TestCaskInfoNeedsAuth(t *testing.T) {
	cask := func(artifacts string) string { return `{"casks":[{"artifacts":[` + artifacts + `]}]}` }
	tests := []struct {
		name string
		json string
		want bool
	}{
		// Trimmed from real `brew info --cask --json=v2` output.
		{"docker-desktop: privileged helper delete", cask(`{"uninstall":[{"launchctl":["com.docker.helper"],"delete":["/Library/PrivilegedHelperTools/com.docker.socket"]}]},{"app":["Docker.app"]}`), true},
		{"visual-studio-code: per-user updater only", cask(`{"uninstall":[{"launchctl":"com.microsoft.VSCode.ShipIt","quit":"com.microsoft.VSCode"}]},{"app":["Visual Studio Code.app"]}`), false},
		{"zoom: pkg", cask(`{"uninstall":[{"signal":["KILL","us.zoom.xos"]}]},{"pkg":["zoomusInstallerFull.pkg"]}`), true},
		{"pkgutil uninstall", cask(`{"uninstall":[{"pkgutil":"us.zoom.pkg.videomeeting"}]}`), true},
		{"kext uninstall", cask(`{"uninstall":[{"kext":"com.example.driver"}]}`), true},
		{"delete list under /Library", cask(`{"uninstall":[{"delete":["~/x","/Library/PrivilegedHelperTools/com.docker.socket"]}]}`), true},
		{"delete string under /Library", cask(`{"uninstall":[{"delete":"/Library/Example"}]}`), true},
		{"postflight steps", cask(`{"postflight_steps":[{"steps":[]}]}`), true},
		{"legacy postflight", cask(`{"postflight":null}`), true},
		{"installer string", cask(`"installer"`), true},
		{"user-level app", cask(`{"uninstall":[{"quit":"com.example.app","delete":"~/Library/Example"}]},{"app":["Example.app"]},{"zap":[{"trash":["/Library/Caches/x"]}]}`), false},
		{"no casks", `{"casks":[]}`, false},
		{"bad json", "not json", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := provider.CaskInfoNeedsAuth(tt.json); got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}
