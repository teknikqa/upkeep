package ui

import "github.com/pterm/pterm"

// Interactive prompts, swappable so tests can drive the menus without a TTY.
// This file holds only pterm glue and is excluded from coverage in codecov.yml.
var (
	isInteractive = IsTTY
	selectOption  = func(label string, options []string) (string, error) {
		return pterm.DefaultInteractiveSelect.WithDefaultText(label).WithOptions(options).Show()
	}
	confirmBool = func(label string, current bool) (bool, error) {
		return pterm.DefaultInteractiveConfirm.WithDefaultText(label).WithDefaultValue(current).Show()
	}
)
