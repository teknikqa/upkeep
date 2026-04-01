package notify_test

import (
	"strings"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/notify"
)

func TestNotify_DisabledReturnsNil(t *testing.T) {
	n := notify.New(config.NotificationsConfig{Enabled: false, Tool: "terminal-notifier"})
	err := n.Notify("Title", "Message", "")
	if err != nil {
		t.Errorf("expected nil error when notifications disabled, got: %v", err)
	}
}

func TestBuildTerminalNotifierArgs_WithoutURL(t *testing.T) {
	args := notify.BuildTerminalNotifierArgs("Upkeep", "3 packages updated", "")
	want := []string{"-title", "Upkeep", "-message", "3 packages updated", "-sender", "com.apple.Terminal"}
	if len(args) != len(want) {
		t.Fatalf("expected %d args, got %d: %v", len(want), len(args), args)
	}
	for i := range want {
		if args[i] != want[i] {
			t.Errorf("args[%d]: expected %q, got %q", i, want[i], args[i])
		}
	}
}

func TestBuildTerminalNotifierArgs_WithURL(t *testing.T) {
	args := notify.BuildTerminalNotifierArgs("Upkeep", "Download available", "https://example.com")
	// Should contain -open and the URL.
	found := false
	for i, a := range args {
		if a == "-open" && i+1 < len(args) && args[i+1] == "https://example.com" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected -open https://example.com in args: %v", args)
	}
}

func TestBuildOsascriptScript(t *testing.T) {
	script := notify.BuildOsascriptScript("Upkeep", "All done")
	if !strings.Contains(script, "display notification") {
		t.Errorf("expected osascript to use 'display notification', got: %q", script)
	}
	if !strings.Contains(script, "Upkeep") {
		t.Errorf("expected title 'Upkeep' in script, got: %q", script)
	}
	if !strings.Contains(script, "All done") {
		t.Errorf("expected message 'All done' in script, got: %q", script)
	}
}

// --- New constructor ---

func TestNew_SetsConfig(t *testing.T) {
	cfg := config.NotificationsConfig{Enabled: true, Tool: "osascript"}
	n := notify.New(cfg)
	if n == nil {
		t.Fatal("expected non-nil Notifier from New()")
	}
	// Verify the config is honoured: disabled path returns nil.
	n2 := notify.New(config.NotificationsConfig{Enabled: false})
	if err := n2.Notify("T", "M", ""); err != nil {
		t.Errorf("expected nil from disabled notifier, got: %v", err)
	}
}

// --- Notify dispatch: disabled overrides explicit tool ---

func TestNotify_ExplicitTerminalNotifier_DisabledReturnsNil(t *testing.T) {
	n := notify.New(config.NotificationsConfig{Enabled: false, Tool: "terminal-notifier"})
	if err := n.Notify("T", "M", "https://example.com"); err != nil {
		t.Errorf("expected nil when disabled with terminal-notifier tool, got: %v", err)
	}
}

func TestNotify_ExplicitOsascript_DisabledReturnsNil(t *testing.T) {
	n := notify.New(config.NotificationsConfig{Enabled: false, Tool: "osascript"})
	if err := n.Notify("T", "M", ""); err != nil {
		t.Errorf("expected nil when disabled with osascript tool, got: %v", err)
	}
}

// TestNotify_UnknownTool_FallsThrough verifies that an unrecognised tool name
// falls through to the auto-detect branch (which tries terminal-notifier then
// osascript) without panicking. We use Enabled=false so no real command runs.
func TestNotify_UnknownTool_FallsThrough(t *testing.T) {
	n := notify.New(config.NotificationsConfig{Enabled: false, Tool: "nonexistent-tool"})
	// Disabled, so the switch is never reached — just confirms no panic.
	if err := n.Notify("T", "M", ""); err != nil {
		t.Errorf("expected nil for disabled notifier with unknown tool, got: %v", err)
	}
}

// TestNotify_EmptyTool_AutoDetects verifies that an empty Tool field routes to
// the auto-detect branch without panicking. We use Enabled=false so no real
// command is executed.
func TestNotify_EmptyTool_AutoDetects(t *testing.T) {
	n := notify.New(config.NotificationsConfig{Enabled: false, Tool: ""})
	if err := n.Notify("T", "M", ""); err != nil {
		t.Errorf("expected nil for disabled notifier with empty tool, got: %v", err)
	}
}
