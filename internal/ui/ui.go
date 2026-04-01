// Package ui provides TUI output wrappers using pterm for upkeep.
package ui

import (
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"golang.org/x/term"

	"github.com/teknikqa/upkeep/internal/provider"
)

// trailingPadRe matches trailing whitespace that may be wrapped inside ANSI
// escape sequences (e.g. "  \033[0m") so we can strip padding pterm adds to
// the last column.
var trailingPadRe = regexp.MustCompile(`\s+(\x1b\[[0-9;]*m\s*)*$`)

// defaultTermWidth is the fallback when terminal width cannot be detected.
const defaultTermWidth = 80

// termWidth returns the current terminal width, or defaultTermWidth if
// detection fails (e.g. not a TTY).
func termWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil || w <= 0 {
		return defaultTermWidth
	}
	return w
}

// WrapPackages splits a list of package names into lines that each fit within
// maxWidth columns when joined by ", ".  Each returned string is a
// comma-separated chunk ready for display.  If maxWidth is too small to hold
// even a single package name, each package gets its own line.
func WrapPackages(pkgs []string, maxWidth int) []string {
	if len(pkgs) == 0 {
		return []string{"-"}
	}
	if maxWidth <= 0 {
		maxWidth = 1
	}

	var lines []string
	var cur strings.Builder
	for _, p := range pkgs {
		if cur.Len() == 0 {
			// First package on this line — always add it regardless of width.
			cur.WriteString(p)
			continue
		}
		// Would appending ", pkg" exceed the budget?
		addition := ", " + p
		if cur.Len()+len(addition) > maxWidth {
			// Flush current line, start a new one.
			lines = append(lines, cur.String())
			cur.Reset()
			cur.WriteString(p)
		} else {
			cur.WriteString(addition)
		}
	}
	if cur.Len() > 0 {
		lines = append(lines, cur.String())
	}
	return lines
}

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

// RenderScanSummaryTable renders a pre-update scan summary table to stdout.
// When a row has PackageGroups, it renders a parent row (total count, no packages)
// followed by indented sub-rows per group (group count + packages).
// Long package lists are wrapped across multiple continuation rows so they
// stay within the Packages column.
// Returns the number of lines printed (for TTY live-table takeover).
func RenderScanSummaryTable(rows []ScanSummaryRow) int {
	if len(rows) == 0 {
		fmt.Println("No providers available or nothing to update.")
		return 1
	}

	if IsTTY() {
		// First pass: build intermediate rows to measure column widths.
		type ttyRow struct {
			provider string
			status   string
			outdated string
			packages []string // pre-split package names (not yet wrapped)
		}
		var intermediate []ttyRow

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
				intermediate = append(intermediate, ttyRow{r.DisplayName, status, outdated, nil})
				for _, sub := range GroupSubRows(r.PackageGroups) {
					intermediate = append(intermediate, ttyRow{
						sub.Label, status, fmt.Sprintf("%d", sub.Count), sub.PkgNames,
					})
				}
			} else {
				intermediate = append(intermediate, ttyRow{
					r.DisplayName, status, outdated, r.Packages,
				})
			}
		}

		// Measure max widths of the first three columns (including headers).
		provW := len("Provider")
		statusW := len("Status")
		outdatedW := len("Outdated")
		for _, ir := range intermediate {
			if len(ir.provider) > provW {
				provW = len(ir.provider)
			}
			if len(ir.status) > statusW {
				statusW = len(ir.status)
			}
			if len(ir.outdated) > outdatedW {
				outdatedW = len(ir.outdated)
			}
		}

		// pterm uses " | " (3 chars) between each column pair:
		//   provW + 3 + statusW + 3 + outdatedW + 3 = prefix before Packages
		prefixWidth := provW + statusW + outdatedW + 9
		tw := termWidth()
		maxPkgWidth := tw - prefixWidth
		if maxPkgWidth < 10 {
			maxPkgWidth = 10
		}

		// Second pass: wrap packages and build final pterm data.
		data := pterm.TableData{
			{"Provider", "Status", "Outdated", "Packages"},
		}
		for _, ir := range intermediate {
			pkgLines := WrapPackages(ir.packages, maxPkgWidth)
			// First line carries the provider/status/outdated columns.
			data = append(data, []string{ir.provider, ir.status, ir.outdated, pkgLines[0]})
			// Continuation lines: blank first 3 columns.
			for _, cont := range pkgLines[1:] {
				data = append(data, []string{"", "", "", cont})
			}
		}

		rendered, _ := pterm.DefaultTable.WithHasHeader().WithData(data).Srender()
		// pterm pads the last column to the max width which causes line
		// wrapping in narrow terminals.  Strip trailing whitespace per line,
		// including whitespace wrapped inside ANSI escape sequences.
		lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
		for _, line := range lines {
			fmt.Println(trailingPadRe.ReplaceAllString(line, ""))
		}
		return len(lines)
	} else {
		// Non-TTY: fixed-width prefix columns (25+1 + 15+1 + 8+1 = 51 chars).
		const nonTTYPrefix = 51
		tw := termWidth()
		maxPkgWidth := tw - nonTTYPrefix
		if maxPkgWidth < 10 {
			maxPkgWidth = 10
		}

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
				fmt.Printf("%-25s %-15s %-8s\n", r.DisplayName, status, outdated)
				for _, sub := range GroupSubRows(r.PackageGroups) {
					pkgLines := WrapPackages(sub.PkgNames, maxPkgWidth)
					fmt.Printf("%25s %-15s %-8d %s\n", sub.Label, status, sub.Count, pkgLines[0])
					for _, cont := range pkgLines[1:] {
						fmt.Printf("%25s %-15s %-8s %s\n", "", "", "", cont)
					}
				}
			} else {
				pkgLines := WrapPackages(r.Packages, maxPkgWidth)
				fmt.Printf("%-25s %-15s %-8s %s\n", r.DisplayName, status, outdated, pkgLines[0])
				for _, cont := range pkgLines[1:] {
					fmt.Printf("%-25s %-15s %-8s %s\n", "", "", "", cont)
				}
			}
		}
	}
	return 0
}

// StatusLine prints a single provider status line.
func StatusLine(w io.Writer, displayName, status string, updated, deferred, skipped, failed int, duration time.Duration) {
	emoji := statusEmoji(status)
	if w == nil {
		w = os.Stdout
	}
	fmt.Fprintf(w, "%s %-20s  updated=%d  deferred=%d  skipped=%d  failed=%d  (%s)\n",
		emoji, displayName, updated, deferred, skipped, failed, duration.Round(time.Millisecond).String())
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

// GroupSubRow represents one sub-group line in a grouped scan summary.
type GroupSubRow struct {
	Label    string   // tree-prefixed group label (e.g. "  ├ code", "  └ cursor")
	Count    int      // number of packages in this group
	Packages string   // comma-separated package names
	PkgNames []string // raw package names (for wrapping)
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
			PkgNames: groups[k],
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
