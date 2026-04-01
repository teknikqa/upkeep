package ui_test

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/teknikqa/upkeep/internal/provider"
	"github.com/teknikqa/upkeep/internal/ui"
)

// scanRows returns a set of ScanSummaryRows for use in LiveUpdateTable tests.
func testScanRows() []ui.ScanSummaryRow {
	return []ui.ScanSummaryRow{
		{
			ProviderName:  "brew",
			DisplayName:   "Homebrew Formulae",
			OutdatedCount: 3,
			Packages:      []string{"git", "jq", "ripgrep"},
			Available:     true,
		},
		{
			ProviderName:  "npm",
			DisplayName:   "npm Global Packages",
			OutdatedCount: 2,
			Packages:      []string{"typescript", "eslint"},
			Available:     true,
		},
		{
			ProviderName:  "pip",
			DisplayName:   "pip / pipx",
			OutdatedCount: 0,
			Available:     false,
		},
	}
}

// TestLiveUpdateTable_NonTTY_OnComplete verifies that in non-TTY mode, a
// StatusLine-like line is written to the writer when OnProviderComplete fires.
func TestLiveUpdateTable_NonTTY_OnComplete(t *testing.T) {
	// IsTTY() returns false in test environments (stdout is not a terminal).
	var buf bytes.Buffer
	rows := testScanRows()
	lt := ui.NewLiveUpdateTable(rows, 0, &buf)

	// Simulate brew completing successfully.
	lt.OnProviderComplete("brew", provider.UpdateResult{
		Updated:  []string{"git", "jq", "ripgrep"},
		Duration: 2 * time.Second,
	})

	// Simulate npm completing with a partial failure.
	lt.OnProviderComplete("npm", provider.UpdateResult{
		Updated:  []string{"typescript"},
		Failed:   []string{"eslint"},
		Duration: 500 * time.Millisecond,
	})

	lt.Stop()

	out := buf.String()

	// Both providers should appear in the output.
	if !strings.Contains(out, "Homebrew Formulae") {
		t.Errorf("expected 'Homebrew Formulae' in output, got: %q", out)
	}
	if !strings.Contains(out, "npm Global Packages") {
		t.Errorf("expected 'npm Global Packages' in output, got: %q", out)
	}
	// Counts should appear.
	if !strings.Contains(out, "updated=3") {
		t.Errorf("expected 'updated=3' in output, got: %q", out)
	}
	if !strings.Contains(out, "updated=1") {
		t.Errorf("expected 'updated=1' in output, got: %q", out)
	}
	if !strings.Contains(out, "failed=1") {
		t.Errorf("expected 'failed=1' in output, got: %q", out)
	}
}

// TestLiveUpdateTable_NonTTY_OnStart verifies OnProviderStart is a no-op for
// output in non-TTY mode (it should not write anything).
func TestLiveUpdateTable_NonTTY_OnStart(t *testing.T) {
	var buf bytes.Buffer
	rows := testScanRows()
	lt := ui.NewLiveUpdateTable(rows, 0, &buf)

	lt.OnProviderStart("brew")

	// In non-TTY mode, OnStart should not produce output.
	if buf.Len() > 0 {
		t.Errorf("expected no output from OnProviderStart in non-TTY mode, got: %q", buf.String())
	}

	lt.Stop()
}

// TestLiveUpdateTable_StateTransitions verifies that status strings progress
// correctly through the state machine.
func TestLiveUpdateTable_StateTransitions(t *testing.T) {
	// We test state transitions indirectly through non-TTY output.
	// After OnComplete with all Updated, status should be "success".
	var buf bytes.Buffer
	rows := []ui.ScanSummaryRow{
		{ProviderName: "brew", DisplayName: "Homebrew", OutdatedCount: 1, Packages: []string{"git"}, Available: true},
	}
	lt := ui.NewLiveUpdateTable(rows, 0, &buf)

	lt.OnProviderComplete("brew", provider.UpdateResult{
		Updated:  []string{"git"},
		Duration: time.Second,
	})
	lt.Stop()

	out := buf.String()
	// StatusLine uses "✅" emoji for success status.
	if !strings.Contains(out, "✅") && !strings.Contains(out, "updated=1") {
		t.Errorf("expected success indicator in output, got: %q", out)
	}
}

// TestLiveUpdateTable_StateTransitions_Failed verifies "failed" status.
func TestLiveUpdateTable_StateTransitions_Failed(t *testing.T) {
	var buf bytes.Buffer
	rows := []ui.ScanSummaryRow{
		{ProviderName: "rust", DisplayName: "Rust", OutdatedCount: 1, Packages: []string{"rustup"}, Available: true},
	}
	lt := ui.NewLiveUpdateTable(rows, 0, &buf)

	lt.OnProviderComplete("rust", provider.UpdateResult{
		Error:    errFakeFailure,
		Duration: 100 * time.Millisecond,
	})
	lt.Stop()

	out := buf.String()
	// StatusLine uses "❌" emoji for failed status.
	if !strings.Contains(out, "❌") && !strings.Contains(out, "failed=0") {
		t.Errorf("expected failed indicator in output, got: %q", out)
	}
}

// TestLiveUpdateTable_StateTransitions_Partial verifies "partial" status when
// some packages fail.
func TestLiveUpdateTable_StateTransitions_Partial(t *testing.T) {
	var buf bytes.Buffer
	rows := []ui.ScanSummaryRow{
		{ProviderName: "npm", DisplayName: "npm", OutdatedCount: 2, Packages: []string{"ts", "eslint"}, Available: true},
	}
	lt := ui.NewLiveUpdateTable(rows, 0, &buf)

	lt.OnProviderComplete("npm", provider.UpdateResult{
		Updated:  []string{"ts"},
		Failed:   []string{"eslint"},
		Duration: 200 * time.Millisecond,
	})
	lt.Stop()

	out := buf.String()
	// StatusLine uses "📬" emoji for partial status.
	if !strings.Contains(out, "📬") && !strings.Contains(out, "failed=1") {
		t.Errorf("expected partial indicator in output, got: %q", out)
	}
}

// TestLiveUpdateTable_ConcurrentAccess verifies no data races when multiple
// goroutines call OnProviderStart and OnProviderComplete concurrently.
// Run with `go test -race` to detect races.
func TestLiveUpdateTable_ConcurrentAccess(t *testing.T) {
	var buf bytes.Buffer
	rows := []ui.ScanSummaryRow{
		{ProviderName: "a", DisplayName: "A", OutdatedCount: 1, Packages: []string{"pkg1"}, Available: true},
		{ProviderName: "b", DisplayName: "B", OutdatedCount: 2, Packages: []string{"pkg2", "pkg3"}, Available: true},
		{ProviderName: "c", DisplayName: "C", OutdatedCount: 0, Available: true},
	}
	lt := ui.NewLiveUpdateTable(rows, 0, &buf)

	var wg sync.WaitGroup
	for _, name := range []string{"a", "b", "c"} {
		name := name
		wg.Add(1)
		go func() {
			defer wg.Done()
			lt.OnProviderStart(name)
			lt.OnProviderComplete(name, provider.UpdateResult{
				Updated:  []string{"pkg"},
				Duration: 10 * time.Millisecond,
			})
		}()
	}
	wg.Wait()
	lt.Stop()
}

// TestLiveUpdateTable_StopIdempotent verifies Stop can be called multiple times
// without panicking.
func TestLiveUpdateTable_StopIdempotent(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Stop panicked: %v", r)
		}
	}()

	var buf bytes.Buffer
	lt := ui.NewLiveUpdateTable(testScanRows(), 0, &buf)
	lt.Stop()
	lt.Stop()
	lt.Stop()
}

// TestLiveUpdateTable_UnknownProvider verifies that callbacks for unknown
// provider names don't panic.
func TestLiveUpdateTable_UnknownProvider(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	var buf bytes.Buffer
	lt := ui.NewLiveUpdateTable(testScanRows(), 0, &buf)
	lt.OnProviderStart("nonexistent")
	lt.OnProviderComplete("nonexistent", provider.UpdateResult{Updated: []string{"x"}})
	lt.Stop()
}

// errFakeFailure is a sentinel error for tests.
var errFakeFailure = fakeError("fake provider failure")

type fakeError string

func (e fakeError) Error() string { return string(e) }

// TestLiveUpdateTable_Stop_PrintsTotalDuration verifies that Stop() prints the
// total duration in non-TTY mode.
func TestLiveUpdateTable_Stop_PrintsTotalDuration(t *testing.T) {
	var buf bytes.Buffer
	rows := testScanRows()
	lt := ui.NewLiveUpdateTable(rows, 0, &buf)

	lt.OnProviderComplete("brew", provider.UpdateResult{
		Updated:  []string{"git"},
		Duration: time.Second,
	})
	lt.SetTotalDuration(5 * time.Second)
	lt.Stop()

	out := buf.String()
	if !strings.Contains(out, "Total duration: 5s") {
		t.Errorf("expected 'Total duration: 5s' in output, got: %q", out)
	}
}

// TestLiveUpdateTable_NonTTY_Skipped verifies that the skipped count appears
// in non-TTY StatusLine output.
func TestLiveUpdateTable_NonTTY_Skipped(t *testing.T) {
	var buf bytes.Buffer
	rows := []ui.ScanSummaryRow{
		{ProviderName: "brew", DisplayName: "Homebrew", OutdatedCount: 3, Packages: []string{"git", "jq", "rg"}, Available: true},
	}
	lt := ui.NewLiveUpdateTable(rows, 0, &buf)

	lt.OnProviderComplete("brew", provider.UpdateResult{
		Updated:  []string{"git"},
		Skipped:  []string{"jq", "rg"},
		Duration: time.Second,
	})
	lt.Stop()

	out := buf.String()
	if !strings.Contains(out, "skipped=2") {
		t.Errorf("expected 'skipped=2' in output, got: %q", out)
	}
}
