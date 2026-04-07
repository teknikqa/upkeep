// Package ui provides TUI output wrappers using pterm for upkeep.
package ui

import (
	"fmt"
	"strings"
	"sync"

	"github.com/pterm/pterm"

	"github.com/teknikqa/upkeep/internal/provider"
)

// providerScanState tracks the scan status for a single provider row.
type providerScanState struct {
	status  string // "scanning" | "done"
	result  *provider.ScanResult
	display string // display name
}

// scanTableRow is an intermediate row used during table rendering.
type scanTableRow struct {
	provider string
	status   string
	outdated string
	packages []string
}

// LiveScanTable renders the scan summary table as a live-updating view during
// the scan phase. On TTY it uses pterm's AreaPrinter to overwrite the table
// in-place as each provider finishes; on non-TTY it is a no-op (the static
// table is printed after all scans complete).
type LiveScanTable struct {
	providerNames     []string // ordered list of provider names
	states            map[string]*providerScanState
	subGroups         map[string][]string // provider name → known sub-group labels
	mu                sync.Mutex
	area              *pterm.AreaPrinter // nil in non-TTY mode
	stopped           bool
	finalRows         []ScanSummaryRow // populated on Stop()
	linesRendered     int              // track lines for live table takeover
	lastRenderedLines int              // lines from the last area.Update call
}

// NewLiveScanTable creates a LiveScanTable that immediately renders a table
// showing all providers in "⏳ scanning" state. providerOrder defines the
// display order; displayNames maps provider name → human-friendly name;
// subGroups maps provider name → list of sub-group labels to show while scanning.
func NewLiveScanTable(providerOrder []string, displayNames map[string]string, subGroups map[string][]string) *LiveScanTable {
	states := make(map[string]*providerScanState, len(providerOrder))
	for _, name := range providerOrder {
		dn := displayNames[name]
		if dn == "" {
			dn = name
		}
		states[name] = &providerScanState{
			status:  "scanning",
			display: dn,
		}
	}

	t := &LiveScanTable{
		providerNames: providerOrder,
		states:        states,
		subGroups:     subGroups,
	}

	if IsTTY() {
		area, err := pterm.DefaultArea.Start()
		if err == nil {
			t.area = area
			t.render()
		}
	}

	return t
}

// OnScanComplete records the scan result for a provider and refreshes the table.
// Called from scanner goroutines — thread-safe.
func (t *LiveScanTable) OnScanComplete(name string, result provider.ScanResult) {
	t.mu.Lock()
	defer t.mu.Unlock()

	s, ok := t.states[name]
	if !ok {
		return
	}
	s.status = "done"
	s.result = &result
	t.render()
}

// Stop finalises the AreaPrinter and returns the final ScanSummaryRows and
// the number of lines rendered. After calling Stop, the static table is left
// on screen for the confirm prompt and live update table to use.
func (t *LiveScanTable) Stop() (rows []ScanSummaryRow, lineCount int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped {
		return t.finalRows, t.linesRendered
	}
	t.stopped = true

	// Build final rows from completed scan states.
	t.finalRows = t.buildSummaryRows()

	if t.area != nil {
		// Do a final render with the full grouped layout, then stop the
		// area printer. pterm's AreaPrinter leaves its last content on
		// screen when stopped, so this becomes the static table — no need
		// to print it again via RenderScanSummaryTable.
		t.renderFinal()
		if err := t.area.Stop(); err != nil {
			_ = err
		}

		// Count the lines the area printer left on screen so that the
		// LiveUpdateTable can erase them later for the execution phase.
		t.linesRendered = t.lastRenderedLines
	}

	return t.finalRows, t.linesRendered
}

// render rebuilds and outputs the table. Must be called with t.mu held.
func (t *LiveScanTable) render() {
	if t.area == nil {
		return
	}

	tw := termWidth()

	var intermediate []scanTableRow

	for _, name := range t.providerNames {
		s := t.states[name]

		if s.status == "scanning" {
			// Show parent row.
			intermediate = append(intermediate, scanTableRow{
				provider: s.display,
				status:   "⏳ scanning",
				outdated: "—",
				packages: nil,
			})
			// Show sub-group rows if known upfront.
			if groups := t.subGroups[name]; len(groups) > 0 {
				for i, g := range groups {
					prefix := "  ├ "
					if i == len(groups)-1 {
						prefix = "  └ "
					}
					intermediate = append(intermediate, scanTableRow{
						provider: prefix + g,
						status:   "⏳ scanning",
						outdated: "—",
						packages: nil,
					})
				}
			}
			continue
		}

		// Provider scan is done — build row from result.
		r := s.result
		status := scanStatusLabel(r.Available, r.Error, len(r.Outdated))
		outdated := fmt.Sprintf("%d", len(r.Outdated))
		if !r.Available {
			outdated = "-"
		}

		pkgNames := make([]string, 0, len(r.Outdated))
		for _, item := range r.Outdated {
			pkgNames = append(pkgNames, item.Name)
		}

		if knownGroups := t.subGroups[name]; len(knownGroups) > 0 {
			intermediate = append(intermediate, scanTableRow{s.display, status, outdated, nil})
			intermediate = append(intermediate, t.allSubGroupRows(name, r.Groups, status)...)
		} else if len(r.Groups) > 0 {
			intermediate = append(intermediate, scanTableRow{s.display, status, outdated, nil})
			intermediate = append(intermediate, t.allSubGroupRows(name, r.Groups, status)...)
		} else {
			intermediate = append(intermediate, scanTableRow{
				s.display, status, outdated, pkgNames,
			})
		}
	}

	// Measure column widths.
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

	prefixWidth := provW + statusW + outdatedW + 9
	maxPkgWidth := tw - prefixWidth
	if maxPkgWidth < 10 {
		maxPkgWidth = 10
	}

	data := pterm.TableData{
		{"Provider", "Status", "Outdated", "Packages"},
	}
	for _, ir := range intermediate {
		pkgLines := WrapPackages(ir.packages, maxPkgWidth)
		data = append(data, []string{ir.provider, ir.status, ir.outdated, pkgLines[0]})
		for _, cont := range pkgLines[1:] {
			data = append(data, []string{"", "", "", cont})
		}
	}

	rendered, err := pterm.DefaultTable.WithHasHeader().WithData(data).Srender()
	if err != nil {
		return
	}

	var sb strings.Builder
	for _, line := range strings.Split(strings.TrimRight(rendered, "\n"), "\n") {
		sb.WriteString(trailingPadRe.ReplaceAllString(line, ""))
		sb.WriteString("\n")
	}

	t.area.Update(sb.String())
}

// renderFinal renders the complete table with group sub-rows expanded,
// matching the layout of RenderScanSummaryTable. Used for the final
// area.Update before stopping. Must be called with t.mu held.
func (t *LiveScanTable) renderFinal() {
	if t.area == nil {
		return
	}

	tw := termWidth()

	var intermediate []scanTableRow

	for _, name := range t.providerNames {
		s := t.states[name]
		if s.result == nil {
			intermediate = append(intermediate, scanTableRow{s.display, "⏭ unavailable", "-", nil})
			continue
		}

		r := s.result
		status := scanStatusLabel(r.Available, r.Error, len(r.Outdated))
		outdated := fmt.Sprintf("%d", len(r.Outdated))
		if !r.Available {
			outdated = "-"
		}

		pkgNames := make([]string, 0, len(r.Outdated))
		for _, item := range r.Outdated {
			pkgNames = append(pkgNames, item.Name)
		}

		if knownGroups := t.subGroups[name]; len(knownGroups) > 0 {
			intermediate = append(intermediate, scanTableRow{s.display, status, outdated, nil})
			intermediate = append(intermediate, t.allSubGroupRows(name, r.Groups, status)...)
		} else if len(r.Groups) > 0 {
			intermediate = append(intermediate, scanTableRow{s.display, status, outdated, nil})
			intermediate = append(intermediate, t.allSubGroupRows(name, r.Groups, status)...)
		} else {
			intermediate = append(intermediate, scanTableRow{s.display, status, outdated, pkgNames})
		}
	}

	// Measure column widths.
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

	prefixWidth := provW + statusW + outdatedW + 9
	maxPkgWidth := tw - prefixWidth
	if maxPkgWidth < 10 {
		maxPkgWidth = 10
	}

	data := pterm.TableData{
		{"Provider", "Status", "Outdated", "Packages"},
	}
	for _, ir := range intermediate {
		pkgLines := WrapPackages(ir.packages, maxPkgWidth)
		data = append(data, []string{ir.provider, ir.status, ir.outdated, pkgLines[0]})
		for _, cont := range pkgLines[1:] {
			data = append(data, []string{"", "", "", cont})
		}
	}

	rendered, err := pterm.DefaultTable.WithHasHeader().WithData(data).Srender()
	if err != nil {
		return
	}

	var sb strings.Builder
	lines := strings.Split(strings.TrimRight(rendered, "\n"), "\n")
	for _, line := range lines {
		sb.WriteString(trailingPadRe.ReplaceAllString(line, ""))
		sb.WriteString("\n")
	}

	t.lastRenderedLines = len(lines)
	t.area.Update(sb.String())
}

// buildSummaryRows converts the final states into ScanSummaryRows.
// Must be called with t.mu held.
func (t *LiveScanTable) buildSummaryRows() []ScanSummaryRow {
	rows := make([]ScanSummaryRow, 0, len(t.providerNames))
	for _, name := range t.providerNames {
		s := t.states[name]
		if s.result == nil {
			// Provider never completed — shouldn't happen but handle gracefully.
			rows = append(rows, ScanSummaryRow{
				ProviderName: name,
				DisplayName:  s.display,
				Available:    false,
			})
			continue
		}
		r := s.result
		pkgs := make([]string, 0, len(r.Outdated))
		for _, item := range r.Outdated {
			pkgs = append(pkgs, item.Name)
		}

		// Merge known sub-groups with scan result groups so all sub-groups
		// appear in the summary (even those with 0 outdated packages).
		groups := r.Groups
		if known := t.subGroups[name]; len(known) > 0 {
			merged := make(map[string][]string, len(known))
			for _, g := range known {
				merged[g] = nil // default: no outdated packages
			}
			for g, p := range r.Groups {
				merged[g] = p
			}
			groups = merged
		}

		rows = append(rows, ScanSummaryRow{
			ProviderName:  name,
			DisplayName:   s.display,
			OutdatedCount: len(r.Outdated),
			Packages:      pkgs,
			PackageGroups: groups,
			Available:     r.Available,
			Error:         r.Error,
		})
	}
	return rows
}

// IsActive returns true if the LiveScanTable is using an AreaPrinter (TTY mode).
func (t *LiveScanTable) IsActive() bool {
	return t.area != nil
}

// allSubGroupRows builds sub-group rows using t.subGroups as the authoritative
// list of labels, merging in actual data from groups (scan result). Sub-groups
// not present in groups are shown with 0 outdated and no packages.
func (t *LiveScanTable) allSubGroupRows(providerName string, groups map[string][]string, status string) []scanTableRow {
	known := t.subGroups[providerName]
	if len(known) == 0 {
		// Fall back to whatever groups the scan returned.
		var rows []scanTableRow
		for _, sub := range GroupSubRows(groups) {
			subStatus := scanStatusLabel(true, nil, sub.Count)
			rows = append(rows, scanTableRow{sub.Label, subStatus, fmt.Sprintf("%d", sub.Count), sub.PkgNames})
		}
		return rows
	}

	var rows []scanTableRow
	for i, g := range known {
		prefix := "  ├ "
		if i == len(known)-1 {
			prefix = "  └ "
		}
		label := prefix + g

		if pkgs, ok := groups[g]; ok && len(pkgs) > 0 {
			rows = append(rows, scanTableRow{label, scanStatusLabel(true, nil, len(pkgs)), fmt.Sprintf("%d", len(pkgs)), pkgs})
		} else {
			rows = append(rows, scanTableRow{label, scanStatusLabel(true, nil, 0), "0", nil})
		}
	}
	return rows
}
