// Package ui provides interactive confirmation prompts.
package ui

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
)

// Confirm asks the user to confirm an action. Returns true if confirmed.
// If yesFlag is true, it auto-confirms and returns true without prompting.
// If stdout is not a TTY, returns false unless yesFlag is set.
func Confirm(message string, yesFlag bool) bool {
	if yesFlag {
		if IsTTY() {
			pterm.Info.Printf("Auto-confirming: %s\n", message)
		} else {
			fmt.Printf("[INFO] Auto-confirming: %s\n", message)
		}
		return true
	}

	if !IsTTY() {
		fmt.Fprintf(os.Stderr, "Non-interactive mode: skipping confirmation for %q — use --yes to auto-confirm\n", message)
		return false
	}

	result, err := pterm.DefaultInteractiveConfirm.
		WithDefaultText(message).
		WithDefaultValue(false).
		Show()
	if err != nil {
		// Fall back to simple stdin prompt.
		return simpleConfirm(message)
	}
	return result
}

// simpleConfirm is a fallback TTY prompt using raw stdin reads.
func simpleConfirm(message string) bool {
	fmt.Printf("%s [y/N]: ", message)
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	line = strings.TrimSpace(strings.ToLower(line))
	return line == "y" || line == "yes"
}
