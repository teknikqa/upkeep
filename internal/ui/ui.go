// Package ui provides TUI output wrappers using pterm for upkeep.
package ui

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"golang.org/x/term"

	"github.com/teknikqa/upkeep/internal/provider"
)

// IsTTY returns true if stdout is a terminal.
func IsTTY() bool {
	return term.IsTerminal(int(os.Stdout.Fd()))
}

// ScanSummaryRow represents one row in the pre-update scan summary table.
type ScanSummaryRow struct {
	ProviderName  string
	DisplayName   string
	OutdatedCount int
	Packages      []string            // first few package names
	PackageGroups map[string][]string // optional: group label → package names
	Available     bool
	Error         error
}

// UpdateSummaryRow represents one row in the final update report.
type UpdateSummaryRow struct {
	ProviderName string
	DisplayName  string
	Updated      int
	Deferred     int
	Skipped      int
	Failed       int
	Duration     time.Duration
	Status       string // "success" | "partial" | "failed" | "skipped" | "unavailable"
	Error        error
}

// RenderScanSummaryTable renders a pre-update scan summary table to stdout.
// When a row has PackageGroups, it renders a parent row (total count, no packages)
// followed by indented sub-rows per group (group count + packages).
func RenderScanSummaryTable(rows []ScanSummaryRow) {
	if len(rows) == 0 {
		fmt.Println("No providers available or nothing to update.")
		return
	}

	if IsTTY() {
		data := pterm.TableData{
			{"Provider", "Status", "Outdated", "Packages"},
		}
		for _, r := range rows {
			status := "✅ available"
			if !r.Available {
				status = "⏭ unavailable"
			} else if r.Error != nil {
				status = "❌ scan error"
			}
			outdated := fmt.Sprintf("%d", r.OutdatedCount)
			if !r.Available {
				outdated = "-"
			}

			if len(r.PackageGroups) > 0 {
				// Parent row: total count, no packages.
				data = append(data, []string{r.DisplayName, status, outdated, ""})
				// Sub-rows per group.
				for _, sub := range GroupSubRows(r.PackageGroups) {
					data = append(data, []string{
						sub.Label, status, fmt.Sprintf("%d", sub.Count), sub.Packages,
					})
				}
			} else {
				data = append(data, []string{r.DisplayName, status, outdated, formatPackageList(r.Packages)})
			}
		}
		_ = pterm.DefaultTable.WithHasHeader().WithData(data).Render()
	} else {
		fmt.Printf("%-25s %-15s %-8s %s\n", "Provider", "Status", "Outdated", "Packages")
		fmt.Printf("%-25s %-15s %-8s %s\n", "--------", "------", "--------", "--------")
		for _, r := range rows {
			status := "available"
			if !r.Available {
				status = "unavailable"
			} else if r.Error != nil {
				status = "scan error"
			}
			outdated := fmt.Sprintf("%d", r.OutdatedCount)
			if !r.Available {
				outdated = "-"
			}

			if len(r.PackageGroups) > 0 {
				// Parent row: total count, no packages.
				fmt.Printf("%-25s %-15s %-8s\n", r.DisplayName, status, outdated)
				// Sub-rows per group.
				for _, sub := range GroupSubRows(r.PackageGroups) {
					fmt.Printf("%25s %-15s %-8d %s\n", sub.Label, status, sub.Count, sub.Packages)
				}
			} else {
				fmt.Printf("%-25s %-15s %-8s %s\n", r.DisplayName, status, outdated, formatPackageList(r.Packages))
			}
		}
	}
}

// RenderFinalReport renders the final update report table.
func RenderFinalReport(rows []UpdateSummaryRow, totalDuration time.Duration) {
	fmt.Println()
	if IsTTY() {
		data := pterm.TableData{
			{"Provider", "Status", "Updated", "Deferred", "Skipped", "Failed", "Duration"},
		}
		for _, r := range rows {
			emoji := statusEmoji(r.Status)
			data = append(data, []string{
				r.DisplayName,
				emoji + " " + r.Status,
				fmt.Sprintf("%d", r.Updated),
				fmt.Sprintf("%d", r.Deferred),
				fmt.Sprintf("%d", r.Skipped),
				fmt.Sprintf("%d", r.Failed),
				r.Duration.Round(time.Millisecond).String(),
			})
		}
		_ = pterm.DefaultTable.WithHasHeader().WithData(data).Render()
	} else {
		fmt.Printf("%-20s %-12s %-8s %-8s %-8s %-8s %s\n",
			"Provider", "Status", "Updated", "Deferred", "Skipped", "Failed", "Duration")
		for _, r := range rows {
			fmt.Printf("%-20s %-12s %-8d %-8d %-8d %-8d %s\n",
				r.DisplayName, r.Status,
				r.Updated, r.Deferred, r.Skipped, r.Failed,
				r.Duration.Round(time.Millisecond).String())
		}
	}
	fmt.Printf("\nTotal duration: %s\n", totalDuration.Round(time.Millisecond).String())
}

// StatusLine prints a single provider status line.
func StatusLine(w io.Writer, displayName, status string, updated, deferred, failed int, duration time.Duration) {
	emoji := statusEmoji(status)
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintf(w, "%s %-20s  updated=%d  deferred=%d  failed=%d  (%s)\n",
		emoji, displayName, updated, deferred, failed, duration.Round(time.Millisecond).String())
}

// ProgressBar creates a deterministic progress bar for N providers.
// Returns an increment function — call it once per completed provider.
func ProgressBar(total int) func() {
	if !IsTTY() || total == 0 {
		return func() {}
	}
	bar, _ := pterm.DefaultProgressbar.WithTotal(total).WithTitle("Updating...").Start()
	return func() {
		bar.Increment()
	}
}

// PrintInfo prints a plain info message, prefixed with ℹ️ on TTY.
func PrintInfo(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if IsTTY() {
		pterm.Info.Println(msg)
	} else {
		fmt.Println("[INFO]", msg)
	}
}

// PrintWarning prints a warning message.
func PrintWarning(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if IsTTY() {
		pterm.Warning.Println(msg)
	} else {
		fmt.Println("[WARN]", msg)
	}
}

// PrintError prints an error message.
func PrintError(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	if IsTTY() {
		pterm.Error.Println(msg)
	} else {
		fmt.Println("[ERROR]", msg)
	}
}

// ScanSummaryRowsFromResults converts scan results to ScanSummaryRows for rendering.
func ScanSummaryRowsFromResults(results map[string]provider.ScanResult, displayNames map[string]string) []ScanSummaryRow {
	rows := make([]ScanSummaryRow, 0, len(results))
	for name, r := range results {
		dn := displayNames[name]
		if dn == "" {
			dn = name
		}
		pkgs := make([]string, 0, len(r.Outdated))
		for _, item := range r.Outdated {
			pkgs = append(pkgs, item.Name)
		}
		rows = append(rows, ScanSummaryRow{
			ProviderName:  name,
			DisplayName:   dn,
			OutdatedCount: len(r.Outdated),
			Packages:      pkgs,
			PackageGroups: r.Groups,
			Available:     r.Available,
			Error:         r.Error,
		})
	}
	return rows
}

// formatPackageList returns package names joined by ", ", or "-" if empty.
func formatPackageList(pkgs []string) string {
	if len(pkgs) == 0 {
		return "-"
	}
	return strings.Join(pkgs, ", ")
}

// GroupSubRow represents one sub-group line in a grouped scan summary.
type GroupSubRow struct {
	Label    string // tree-prefixed group label (e.g. "  ├ code", "  └ cursor")
	Count    int    // number of packages in this group
	Packages string // comma-separated package names
}

// GroupSubRows expands a Groups map into sorted sub-rows for table rendering.
// Each group becomes a row with a tree-style prefix (├ for intermediate, └ for last).
func GroupSubRows(groups map[string][]string) []GroupSubRow {
	if len(groups) == 0 {
		return nil
	}

	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Filter out empty groups first so we know which is last.
	var nonEmpty []string
	for _, k := range keys {
		if len(groups[k]) > 0 {
			nonEmpty = append(nonEmpty, k)
		}
	}

	rows := make([]GroupSubRow, 0, len(nonEmpty))
	for i, k := range nonEmpty {
		prefix := "  ├ "
		if i == len(nonEmpty)-1 {
			prefix = "  └ "
		}
		rows = append(rows, GroupSubRow{
			Label:    prefix + k,
			Count:    len(groups[k]),
			Packages: strings.Join(groups[k], ", "),
		})
	}
	return rows
}

// FormatGroupedPackageList renders sub-grouped packages as "group1: pkg1, pkg2; group2: pkg3".
// Groups with no packages are omitted. Group labels are sorted for deterministic output.
func FormatGroupedPackageList(groups map[string][]string) string {
	if len(groups) == 0 {
		return "-"
	}
	// Sort group keys for deterministic output.
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var parts []string
	for _, k := range keys {
		pkgs := groups[k]
		if len(pkgs) == 0 {
			continue
		}
		parts = append(parts, k+": "+strings.Join(pkgs, ", "))
	}
	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, "; ")
}

func statusEmoji(status string) string {
	switch status {
	case "success":
		return "✅"
	case "partial":
		return "📬"
	case "failed":
		return "❌"
	case "skipped", "unavailable":
		return "⏭"
	default:
		return "❓"
	}
}
