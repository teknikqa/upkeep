// Package ui provides TUI output wrappers using pterm for upkeep.
package ui

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/pterm/pterm"

	"github.com/teknikqa/upkeep/internal/provider"
)

// providerUpdateState tracks live progress for a single provider.
type providerUpdateState struct {
	status        string // "pending" | "updating" | "success" | "partial" | "failed" | "unavailable"
	updatedCount  int
	deferredCount int
	skippedCount  int
	failedCount   int
	duration      time.Duration
	startTime     time.Time // set when status becomes "updating"
	currentPkg    string    // package currently being processed (for footer)
	packages      []string  // accumulated package names (all outcomes) for the Packages column
}

// LiveUpdateTable renders the scan summary table as a live-updating view during
// the update phase. On TTY it uses pterm's AreaPrinter to overwrite the table
// in-place; on non-TTY it falls back to per-completion StatusLine output.
type LiveUpdateTable struct {
	rows          []ScanSummaryRow // immutable initial scan data
	states        map[string]*providerUpdateState
	mu            sync.Mutex
	area          *pterm.AreaPrinter // nil in non-TTY mode
	writer        io.Writer          // for non-TTY fallback output
	stopped       bool
	totalDuration time.Duration
	tickerStop    chan struct{} // signals the duration ticker to stop
	tickerDone    chan struct{} // closed when the ticker goroutine exits
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
		rows:       rows,
		states:     states,
		writer:     w,
		tickerStop: make(chan struct{}),
		tickerDone: make(chan struct{}),
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
			// Start the duration ticker to update elapsed times every 100ms.
			go t.runTicker()
		} else {
			close(t.tickerDone)
		}
	} else {
		close(t.tickerDone)
	}

	return t
}

// runTicker periodically re-renders the table to update live durations.
// Runs in its own goroutine; stopped by closing t.tickerStop.
func (t *LiveUpdateTable) runTicker() {
	defer close(t.tickerDone)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-t.tickerStop:
			return
		case <-ticker.C:
			t.mu.Lock()
			if !t.stopped {
				t.render()
			}
			t.mu.Unlock()
		}
	}
}

// OnProviderStart marks a provider as currently updating and refreshes the table.
// Called from executor goroutines — thread-safe.
func (t *LiveUpdateTable) OnProviderStart(name string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if s, ok := t.states[name]; ok {
		s.status = "updating"
		s.startTime = time.Now()
	}
	t.render()
}

// OnPackageProgress records incremental progress for a single package within a
// provider and refreshes the table. Called from executor goroutines — thread-safe.
func (t *LiveUpdateTable) OnPackageProgress(providerName string, progress provider.PackageProgress) {
	t.mu.Lock()
	defer t.mu.Unlock()

	s, ok := t.states[providerName]
	if !ok {
		return
	}

	switch progress.Status {
	case provider.PackageStarting:
		// Record which package is about to be processed (for the footer).
		s.currentPkg = progress.Name
		if IsTTY() {
			t.render()
		}
		return
	case provider.PackageUpdated:
		s.updatedCount++
	case provider.PackageFailed:
		s.failedCount++
	case provider.PackageDeferred:
		s.deferredCount++
	case provider.PackageSkipped:
		s.skippedCount++
	}

	// Record the package name for the Packages column.
	if progress.Name != "" {
		s.packages = append(s.packages, progress.Name)
	}

	// Clear the current package indicator since this one just finished.
	s.currentPkg = ""

	if IsTTY() {
		t.render()
	}
}

// OnPackageStart records which package is about to be processed for the footer.
// Called from executor goroutines — thread-safe.
func (t *LiveUpdateTable) OnPackageStart(providerName string, pkgName string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if s, ok := t.states[providerName]; ok {
		s.currentPkg = pkgName
	}
	// No render here — the ticker will pick it up.
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

	// Use the final counts from the result (authoritative over incremental).
	s.updatedCount = len(result.Updated)
	s.deferredCount = len(result.Deferred)
	s.skippedCount = len(result.Skipped)
	s.failedCount = len(result.Failed)
	s.duration = result.Duration
	s.currentPkg = ""

	// Build the authoritative package list from the final result.
	s.packages = buildFinalPackages(result)

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
		StatusLine(t.writer, t.displayNameFor(name), s.status, s.updatedCount, s.deferredCount, s.skippedCount, s.failedCount, s.duration)
	}
}

// SetTotalDuration records the overall update duration for display on Stop.
func (t *LiveUpdateTable) SetTotalDuration(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.totalDuration = d
}

// Stop finalises the AreaPrinter, leaving the last-rendered table on screen,
// and prints the total duration line below. Safe to call multiple times.
func (t *LiveUpdateTable) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.stopped {
		return
	}
	t.stopped = true

	// Stop the ticker goroutine.
	close(t.tickerStop)

	if t.area != nil {
		// Wait for ticker to fully exit before final render to avoid races.
		t.mu.Unlock()
		<-t.tickerDone
		t.mu.Lock()

		// Do a final render with all final states before stopping.
		t.render()
		if err := t.area.Stop(); err != nil {
			// Ignore stop errors — best effort.
			_ = err
		}
		fmt.Fprintf(os.Stdout, "\nTotal duration: %s\n", t.totalDuration.Round(time.Millisecond))
	} else {
		// Non-TTY: print total duration to writer.
		fmt.Fprintf(t.writer, "\nTotal duration: %s\n", t.totalDuration.Round(time.Millisecond))
	}
}

// render rebuilds and outputs the table. Must be called with t.mu held.
// On TTY it calls area.Update; on non-TTY it is a no-op (StatusLine is used instead).
func (t *LiveUpdateTable) render() {
	if t.area == nil {
		return
	}

	tw := termWidth()

	// Build intermediate rows — same logic as RenderScanSummaryTable but with
	// report columns (Updated, Deferred, Skipped, Failed, Duration).
	type iRow struct {
		provider string
		status   string
		outdated string
		upd      string
		def      string
		skip     string
		fail     string
		dur      string
		packages []string
	}
	var intermediate []iRow

	for _, r := range t.rows {
		s := t.states[r.ProviderName]

		status, outdated := t.rowStatusAndOutdated(r, s)
		upd, def, skip, fail, dur := t.reportColumns(s)

		// Use accumulated package names from state when available (updating/completed);
		// fall back to scan-time r.Packages for pending providers.
		pkgs := r.Packages
		if s != nil && s.status != "pending" && s.status != "unavailable" && len(s.packages) > 0 {
			pkgs = s.packages
		}

		if len(r.PackageGroups) > 0 {
			intermediate = append(intermediate, iRow{r.DisplayName, status, outdated, upd, def, skip, fail, dur, nil})
			for _, sub := range GroupSubRows(r.PackageGroups) {
				// Sub-rows: carry parent status, show per-group count, blank report cols.
				intermediate = append(intermediate, iRow{
					sub.Label, status, fmt.Sprintf("%d", sub.Count), "", "", "", "", "", sub.PkgNames,
				})
			}
		} else {
			intermediate = append(intermediate, iRow{
				r.DisplayName, status, outdated, upd, def, skip, fail, dur, pkgs,
			})
		}
	}

	// Measure column widths.
	provW := len("Provider")
	statusW := len("Status")
	outdatedW := len("Outdated")
	updW := len("Upd")
	defW := len("Def")
	skipW := len("Skip")
	failW := len("Fail")
	durW := len("Dur")
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
		if len(ir.upd) > updW {
			updW = len(ir.upd)
		}
		if len(ir.def) > defW {
			defW = len(ir.def)
		}
		if len(ir.skip) > skipW {
			skipW = len(ir.skip)
		}
		if len(ir.fail) > failW {
			failW = len(ir.fail)
		}
		if len(ir.dur) > durW {
			durW = len(ir.dur)
		}
	}

	// pterm uses " | " (3 chars) between each column pair.
	// 9 columns → 8 separators × 3 = 24.
	prefixWidth := provW + statusW + outdatedW + updW + defW + skipW + failW + durW + 24
	maxPkgWidth := tw - prefixWidth
	if maxPkgWidth < 10 {
		maxPkgWidth = 10
	}

	data := pterm.TableData{
		{"Provider", "Status", "Outdated", "Upd", "Def", "Skip", "Fail", "Dur", "Packages"},
	}
	for _, ir := range intermediate {
		pkgLines := WrapPackages(ir.packages, maxPkgWidth)
		data = append(data, []string{ir.provider, ir.status, ir.outdated, ir.upd, ir.def, ir.skip, ir.fail, ir.dur, pkgLines[0]})
		for _, cont := range pkgLines[1:] {
			data = append(data, []string{"", "", "", "", "", "", "", "", cont})
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

	// Append the active packages footer.
	footer := t.activePackagesFooter(tw)
	if footer != "" {
		sb.WriteString(footer)
	}

	t.area.Update(strings.TrimRight(sb.String(), "\n"))
}

// activePackagesFooter builds a footer line showing which packages are currently
// being updated. Returns empty string if nothing is actively updating.
func (t *LiveUpdateTable) activePackagesFooter(maxWidth int) string {
	// Collect active package names with their provider display name.
	type activePkg struct {
		providerDisplay string
		pkgName         string
	}
	var active []activePkg

	for _, r := range t.rows {
		s := t.states[r.ProviderName]
		if s == nil || s.status != "updating" {
			continue
		}
		if s.currentPkg != "" {
			active = append(active, activePkg{r.DisplayName, s.currentPkg})
		} else {
			// Provider is updating but no specific package — show provider name.
			active = append(active, activePkg{r.DisplayName, ""})
		}
	}

	if len(active) == 0 {
		return ""
	}

	// Sort for stable output.
	sort.Slice(active, func(i, j int) bool {
		return active[i].providerDisplay < active[j].providerDisplay
	})

	// Build the footer.
	var parts []string
	for _, a := range active {
		if a.pkgName != "" {
			parts = append(parts, fmt.Sprintf("%s → %s", a.providerDisplay, a.pkgName))
		} else {
			parts = append(parts, a.providerDisplay)
		}
	}

	line := "⏳ Updating: " + strings.Join(parts, ", ")

	// Truncate if too wide.
	if maxWidth > 0 && len(line) > maxWidth {
		if maxWidth > 4 {
			line = line[:maxWidth-3] + "..."
		} else {
			line = line[:maxWidth]
		}
	}

	return "\n" + line
}

// reportColumns returns the 5 report column strings for a provider state.
// For "updating" providers, shows live incremental values and elapsed time.
// Returns "—" for pending/unavailable providers and actual values for completed ones.
func (t *LiveUpdateTable) reportColumns(s *providerUpdateState) (upd, def, skip, fail, dur string) {
	if s == nil {
		return "—", "—", "—", "—", "—"
	}
	switch s.status {
	case "pending", "unavailable":
		return "—", "—", "—", "—", "—"
	case "updating":
		// Show live incremental counts and elapsed time.
		elapsed := time.Since(s.startTime).Round(100 * time.Millisecond)
		return fmt.Sprintf("%d", s.updatedCount),
			fmt.Sprintf("%d", s.deferredCount),
			fmt.Sprintf("%d", s.skippedCount),
			fmt.Sprintf("%d", s.failedCount),
			elapsed.String()
	default: // "success", "partial", "failed"
		return fmt.Sprintf("%d", s.updatedCount),
			fmt.Sprintf("%d", s.deferredCount),
			fmt.Sprintf("%d", s.skippedCount),
			fmt.Sprintf("%d", s.failedCount),
			s.duration.Round(time.Millisecond).String()
	}
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
		// Show decremented outdated count based on incremental progress.
		remaining := r.OutdatedCount - s.updatedCount - s.failedCount - s.deferredCount - s.skippedCount
		if remaining < 0 {
			remaining = 0
		}
		outdated = fmt.Sprintf("%d", remaining)
	case "success":
		status = "✅ success"
		remaining := r.OutdatedCount - s.updatedCount - s.failedCount - s.skippedCount
		if remaining < 0 {
			remaining = 0
		}
		outdated = fmt.Sprintf("%d", remaining)
	case "partial":
		status = "📬 partial"
		remaining := r.OutdatedCount - s.updatedCount - s.failedCount - s.skippedCount
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

// buildFinalPackages combines all outcome lists from an UpdateResult into a
// single deduplicated package list for the Packages column.
func buildFinalPackages(result provider.UpdateResult) []string {
	total := len(result.Updated) + len(result.Deferred) + len(result.Skipped) + len(result.Failed)
	if total == 0 {
		return nil
	}
	pkgs := make([]string, 0, total)
	pkgs = append(pkgs, result.Updated...)
	pkgs = append(pkgs, result.Deferred...)
	pkgs = append(pkgs, result.Skipped...)
	pkgs = append(pkgs, result.Failed...)
	return pkgs
}
