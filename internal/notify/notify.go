// Package notify provides macOS Notification Center integration.
package notify

import (
	"fmt"
	"os/exec"

	"github.com/teknikqa/upkeep/internal/config"
)

// Notifier sends macOS notifications.
type Notifier struct {
	cfg config.NotificationsConfig
}

// New creates a Notifier using the given config.
func New(cfg config.NotificationsConfig) *Notifier {
	return &Notifier{cfg: cfg}
}

// Notify sends a macOS notification with the given title, message, and optional URL.
// Returns nil if notifications are disabled.
func (n *Notifier) Notify(title, message, url string) error {
	if !n.cfg.Enabled {
		return nil
	}

	switch n.cfg.Tool {
	case "terminal-notifier":
		return n.notifyTerminalNotifier(title, message, url)
	case "osascript":
		return n.notifyOsascript(title, message)
	default:
		// Auto-detect: try terminal-notifier first, fall back to osascript.
		if _, err := exec.LookPath("terminal-notifier"); err == nil {
			return n.notifyTerminalNotifier(title, message, url)
		}
		return n.notifyOsascript(title, message)
	}
}

// notifyTerminalNotifier sends a notification using terminal-notifier.
func (n *Notifier) notifyTerminalNotifier(title, message, url string) error {
	args := []string{
		"-title", title,
		"-message", message,
		"-sender", "com.apple.Terminal",
	}
	if url != "" {
		args = append(args, "-open", url)
	}
	cmd := exec.Command("terminal-notifier", args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("terminal-notifier: %w (output: %s)", err, out)
	}
	return nil
}

// notifyOsascript sends a notification using osascript.
func (n *Notifier) notifyOsascript(title, message string) error {
	script := fmt.Sprintf(`display notification %q with title %q`, message, title)
	cmd := exec.Command("osascript", "-e", script)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("osascript: %w (output: %s)", err, out)
	}
	return nil
}

// BuildTerminalNotifierArgs returns the args that would be passed to terminal-notifier.
// Exported for testing without actually running the command.
func BuildTerminalNotifierArgs(title, message, url string) []string {
	args := []string{
		"-title", title,
		"-message", message,
		"-sender", "com.apple.Terminal",
	}
	if url != "" {
		args = append(args, "-open", url)
	}
	return args
}

// BuildOsascriptScript returns the AppleScript that would be executed.
// Exported for testing without actually running the command.
func BuildOsascriptScript(title, message string) string {
	return fmt.Sprintf(`display notification %q with title %q`, message, title)
}
