// Package ui provides TUI output wrappers using pterm for upkeep.
package ui

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"

	"github.com/teknikqa/upkeep/internal/provider"
)

// providerUpdateState tracks live progress for a single provider.
type providerUpdateState struct {
	status       string // "pending" | "updating" | "success" | "partial" | "failed" | "unavailable"
	updatedCount int
	failedCount  int
	duration     time.Duration
}

// LiveUpdateTable renders the scan summary table as a live-updating view during
// the update phase. On TTY it uses pterm's AreaPrinter to overwrite the table
// in-place; on non-TTY it falls back to per-completion StatusLine output.
type LiveUpdateTable struct {
	rows    []ScanSummaryRow // immutable initial scan data
	states  map[string]*providerUpdateState
	mu      sync.Mutex
	area    *pterm.AreaPrinter // nil in non-TTY mode
	writer  io.Writer          // for non-TTY fallback output
	stopped bool
}

// NewLiveUpdateTable creates a LiveUpdateTable from the scan summary rows.
// scanTableLines is the number of lines the static scan table occupies (from
// RenderScanSummaryTable); on TTY the constructor erases those lines and
// replaces them with a live-updating AreaPrinter.
// On non-TTY it stores w for StatusLine fallback output.
func NewLiveUpdateTable(rows []ScanSummaryRow, scanTableLines int, w io.Writer) *LiveUpdateTable {
	if w == nil {
		w = io.Discard
	}

	states := make(map[string]*providerUpdateState, len(rows))
	for _, r := range rows {
		status := "pending"
		if !r.Available {
			status = "unavailable"
		}
		states[r.ProviderName] = &providerUpdateState{
			status: status,
		}
	}

	t := &LiveUpdateTable{
		rows:   rows,
		states: states,
		writer: w,
	}

	if IsTTY() {
		// Erase the static scan summary table so the AreaPrinter replaces it
		// in-place. Also erase the confirm prompt line(s) above.
		// We erase scanTableLines (the table) + 1 (the confirm prompt line).
		eraseLines := scanTableLines + 1
		for i := 0; i < eraseLines; i++ {
			// Move cursor up one line and clear it.
			fmt.Print("\033[A\033[2K")
		}

		area, err := pterm.DefaultArea.Start()
		if err == nil {
			t.area = area
			t.render()
		}
	}

	return t
}

// OnProviderStart marks a provider as currently updating and refreshes the table.
// Called from executor goroutines — thread-safe.
func (t *LiveUpdateTable) OnProviderStart(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if s, ok := t.states[name]; ok {
		s.status = "updating"
	}
	t.render()
}

// OnProviderComplete records the final result for a provider and refreshes the table.
// Called from executor goroutines — thread-safe.
func (t *LiveUpdateTable) OnProviderComplete(name string, result provider.UpdateResult) {
	t.mu.Lock()
	defer t.mu.Unlock()

	s, ok := t.states[name]
	if !ok {
		return
	}

	s.updatedCount = len(result.Updated)
	s.failedCount = len(result.Failed)
	s.duration = result.Duration

	switch {
	case result.Error != nil && s.updatedCount == 0 && len(result.Deferred) == 0:
		s.status = "failed"
	case s.failedCount > 0 || result.Error != nil:
		s.status = "partial"
	case len(result.Deferred) > 0 && s.updatedCount == 0 && s.failedCount == 0:
		s.status = "partial"
	default:
		s.status = "success"
	}

	if IsTTY() {
		t.render()
	} else {
		// Non-TTY fallback: emit a StatusLine to the writer.
		StatusLine(t.writer, t.displayNameFor(name), s.status, s.updatedCount, len(result.Deferred), s.failedCount, s.duration)
	}
}

// Stop finalises the AreaPrinter, leaving the last-rendered table on screen.
// Safe to call multiple times.
func (t *LiveUpdateTable) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped {
		return
	}
	t.stopped = true

	if t.area != nil {
		// Do a final render with all final states before stopping.
		t.render()
		if err := t.area.Stop(); err != nil {
			// Ignore stop errors — best effort.
			_ = err
		}
	}
}

// render rebuilds and outputs the table. Must be called with t.mu held.
// On TTY it calls area.Update; on non-TTY it is a no-op (StatusLine is used instead).
func (t *LiveUpdateTable) render() {
	if t.area == nil {
		return
	}

	tw := termWidth()

	// Build intermediate rows — same logic as RenderScanSummaryTable.
	type iRow struct {
		provider string
		status   string
		outdated string
		packages []string
	}
	var intermediate []iRow

	for _, r := range t.rows {
		s := t.states[r.ProviderName]

		status, outdated := t.rowStatusAndOutdated(r, s)

		if len(r.PackageGroups) > 0 {
			intermediate = append(intermediate, iRow{r.DisplayName, status, outdated, nil})
			for _, sub := range GroupSubRows(r.PackageGroups) {
				// For sub-rows, carry the parent status and show per-group count.
				intermediate = append(intermediate, iRow{
					sub.Label, status, fmt.Sprintf("%d", sub.Count), sub.PkgNames,
				})
			}
		} else {
			intermediate = append(intermediate, iRow{
				r.DisplayName, status, outdated, r.Packages,
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

	// Strip trailing whitespace per line (same as RenderScanSummaryTable).
	var sb strings.Builder
	for _, line := range strings.Split(strings.TrimRight(rendered, "\n"), "\n") {
		sb.WriteString(trailingPadRe.ReplaceAllString(line, ""))
		sb.WriteString("\n")
	}

	t.area.Update(strings.TrimRight(sb.String(), "\n"))
}

// rowStatusAndOutdated returns the Status and Outdated column strings for a row.
func (t *LiveUpdateTable) rowStatusAndOutdated(r ScanSummaryRow, s *providerUpdateState) (status, outdated string) {
	if s == nil || !r.Available {
		return "⏭ unavailable", "-"
	}
	if r.Error != nil {
		return "❌ scan error", "-"
	}

	switch s.status {
	case "updating":
		status = "🔄 updating"
		outdated = fmt.Sprintf("%d", r.OutdatedCount)
	case "success":
		status = "✅ success"
		remaining := r.OutdatedCount - s.updatedCount - s.failedCount
		if remaining < 0 {
			remaining = 0
		}
		outdated = fmt.Sprintf("%d", remaining)
	case "partial":
		status = "📬 partial"
		remaining := r.OutdatedCount - s.updatedCount - s.failedCount
		if remaining < 0 {
			remaining = 0
		}
		outdated = fmt.Sprintf("%d", remaining)
	case "failed":
		status = "❌ failed"
		outdated = fmt.Sprintf("%d", r.OutdatedCount)
	case "unavailable":
		status = "⏭ unavailable"
		outdated = "-"
	default: // "pending"
		status = "✅ available"
		outdated = fmt.Sprintf("%d", r.OutdatedCount)
	}
	return status, outdated
}

// displayNameFor returns the display name for a provider from the rows list.
func (t *LiveUpdateTable) displayNameFor(name string) string {
	for _, r := range t.rows {
		if r.ProviderName == name {
			return r.DisplayName
		}
	}
	return name
}
