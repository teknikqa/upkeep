package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/pterm/pterm"
)

// editBool prompts the user to toggle a boolean value.
func editBool(label string, current bool) (bool, error) {
	result, err := pterm.DefaultInteractiveConfirm.
		WithDefaultText(label).
		WithDefaultValue(current).
		Show()
	if err != nil {
		return current, fmt.Errorf("editing %s: %w", label, err)
	}
	return result, nil
}

// editInt prompts the user to enter an integer within [min, max].
func editInt(label string, current, min, max int) (int, error) {
	for {
		result, err := pterm.DefaultInteractiveTextInput.
			WithDefaultText(fmt.Sprintf("%s (%d–%d)", label, min, max)).
			WithDefaultValue(strconv.Itoa(current)).
			Show()
		if err != nil {
			return current, fmt.Errorf("editing %s: %w", label, err)
		}

		val, err := strconv.Atoi(strings.TrimSpace(result))
		if err != nil {
			pterm.Error.Printfln("Invalid number: %s", result)
			continue
		}
		if val < min || val > max {
			pterm.Error.Printfln("Value must be between %d and %d", min, max)
			continue
		}
		return val, nil
	}
}

// editString prompts the user to enter a free-text string value.
func editString(label string, current string) (string, error) {
	result, err := pterm.DefaultInteractiveTextInput.
		WithDefaultText(label).
		WithDefaultValue(current).
		Show()
	if err != nil {
		return current, fmt.Errorf("editing %s: %w", label, err)
	}
	return strings.TrimSpace(result), nil
}

// editEnum prompts the user to select from a fixed set of options.
func editEnum(label string, current string, options []string) (string, error) {
	result, err := pterm.DefaultInteractiveSelect.
		WithDefaultText(label).
		WithOptions(options).
		WithDefaultOption(current).
		Show()
	if err != nil {
		return current, fmt.Errorf("editing %s: %w", label, err)
	}
	return result, nil
}

// editStringSlice lets the user add/remove items from a string slice.
func editStringSlice(label string, current []string) ([]string, error) {
	// Work on a copy.
	items := make([]string, len(current))
	copy(items, current)

	for {
		header := fmt.Sprintf("%s: %s", label, formatSlice(items))
		options := []string{"Add item", "Remove item", "Back"}

		choice, err := pterm.DefaultInteractiveSelect.
			WithDefaultText(header).
			WithOptions(options).
			Show()
		if err != nil {
			return items, fmt.Errorf("editing %s: %w", label, err)
		}

		switch choice {
		case "Add item":
			val, err := pterm.DefaultInteractiveTextInput.
				WithDefaultText("New item").
				Show()
			if err != nil {
				return items, err
			}
			val = strings.TrimSpace(val)
			if val != "" {
				items = append(items, val)
			}

		case "Remove item":
			if len(items) == 0 {
				pterm.Warning.Println("List is empty")
				continue
			}
			removeOpts := append([]string{}, items...)
			removeOpts = append(removeOpts, "Cancel")
			toRemove, err := pterm.DefaultInteractiveSelect.
				WithDefaultText("Select item to remove").
				WithOptions(removeOpts).
				Show()
			if err != nil || toRemove == "Cancel" {
				continue
			}
			items = removeFromSlice(items, toRemove)

		case "Back":
			return items, nil
		}
	}
}

// editMapStringBool lets the user add/remove/toggle entries in a map[string]bool.
func editMapStringBool(label string, current map[string]bool) (map[string]bool, error) {
	// Work on a copy.
	m := make(map[string]bool, len(current))
	for k, v := range current {
		m[k] = v
	}

	for {
		header := fmt.Sprintf("%s: %s", label, formatMap(m))
		options := []string{"Add entry", "Toggle entry", "Remove entry", "Back"}

		choice, err := pterm.DefaultInteractiveSelect.
			WithDefaultText(header).
			WithOptions(options).
			Show()
		if err != nil {
			return m, fmt.Errorf("editing %s: %w", label, err)
		}

		switch choice {
		case "Add entry":
			key, err := pterm.DefaultInteractiveTextInput.
				WithDefaultText("Key name").
				Show()
			if err != nil {
				return m, err
			}
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			val, err := pterm.DefaultInteractiveConfirm.
				WithDefaultText(fmt.Sprintf("Value for %q", key)).
				WithDefaultValue(true).
				Show()
			if err != nil {
				return m, err
			}
			m[key] = val

		case "Toggle entry":
			if len(m) == 0 {
				pterm.Warning.Println("Map is empty")
				continue
			}
			keys := mapKeys(m)
			togOpts := make([]string, len(keys)+1)
			copy(togOpts, keys)
			togOpts[len(keys)] = "Cancel"
			toToggle, err := pterm.DefaultInteractiveSelect.
				WithDefaultText("Select entry to toggle").
				WithOptions(togOpts).
				Show()
			if err != nil || toToggle == "Cancel" {
				continue
			}
			m[toToggle] = !m[toToggle]

		case "Remove entry":
			if len(m) == 0 {
				pterm.Warning.Println("Map is empty")
				continue
			}
			keys := mapKeys(m)
			remOpts := make([]string, len(keys)+1)
			copy(remOpts, keys)
			remOpts[len(keys)] = "Cancel"
			toRemove, err := pterm.DefaultInteractiveSelect.
				WithDefaultText("Select entry to remove").
				WithOptions(remOpts).
				Show()
			if err != nil || toRemove == "Cancel" {
				continue
			}
			delete(m, toRemove)

		case "Back":
			return m, nil
		}
	}
}

// formatMenuLabel formats a menu label showing the current value.
func formatMenuLabel(label string, value any) string {
	switch v := value.(type) {
	case bool:
		if v {
			return fmt.Sprintf("%s [enabled]", label)
		}
		return fmt.Sprintf("%s [disabled]", label)
	case int:
		return fmt.Sprintf("%s [%d]", label, v)
	case string:
		return fmt.Sprintf("%s [%s]", label, v)
	case []string:
		return fmt.Sprintf("%s [%s]", label, formatSlice(v))
	case map[string]bool:
		return fmt.Sprintf("%s [%s]", label, formatMap(v))
	default:
		return fmt.Sprintf("%s [%v]", label, v)
	}
}

// formatSlice returns a compact string representation of a string slice.
func formatSlice(items []string) string {
	if len(items) == 0 {
		return "none"
	}
	if len(items) <= 3 {
		return strings.Join(items, ", ")
	}
	return strings.Join(items[:3], ", ") + fmt.Sprintf(" (+%d more)", len(items)-3)
}

// formatMap returns a compact string representation of a map[string]bool.
func formatMap(m map[string]bool) string {
	if len(m) == 0 {
		return "none"
	}
	parts := make([]string, 0, len(m))
	for k, v := range m {
		if v {
			parts = append(parts, k+":on")
		} else {
			parts = append(parts, k+":off")
		}
	}
	return strings.Join(parts, ", ")
}

// removeFromSlice removes the first occurrence of val from items.
func removeFromSlice(items []string, val string) []string {
	for i, item := range items {
		if item == val {
			return append(items[:i], items[i+1:]...)
		}
	}
	return items
}

// mapKeys returns the keys of a map[string]bool sorted for display.
func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
