package ui

import (
	"testing"
	"time"
)

// TestReportColumns_Pending verifies that pending providers show dashes.
func TestReportColumns_Pending(t *testing.T) {
	lt := &LiveUpdateTable{
		states: map[string]*providerUpdateState{},
	}

	s := &providerUpdateState{status: "pending"}
	upd, def, skip, fail, dur := lt.reportColumns(s)
	if upd != "—" || def != "—" || skip != "—" || fail != "—" || dur != "—" {
		t.Errorf("pending: expected all dashes, got upd=%q def=%q skip=%q fail=%q dur=%q", upd, def, skip, fail, dur)
	}
}

// TestReportColumns_Unavailable verifies that unavailable providers show dashes.
func TestReportColumns_Unavailable(t *testing.T) {
	lt := &LiveUpdateTable{
		states: map[string]*providerUpdateState{},
	}

	s := &providerUpdateState{status: "unavailable"}
	upd, def, skip, fail, dur := lt.reportColumns(s)
	if upd != "—" || def != "—" || skip != "—" || fail != "—" || dur != "—" {
		t.Errorf("unavailable: expected all dashes, got upd=%q def=%q skip=%q fail=%q dur=%q", upd, def, skip, fail, dur)
	}
}

// TestReportColumns_Updating verifies that updating providers show live values.
func TestReportColumns_Updating(t *testing.T) {
	lt := &LiveUpdateTable{
		states: map[string]*providerUpdateState{},
	}

	s := &providerUpdateState{
		status:        "updating",
		updatedCount:  3,
		deferredCount: 1,
		skippedCount:  2,
		failedCount:   0,
		startTime:     time.Now().Add(-5 * time.Second),
	}
	upd, def, skip, fail, dur := lt.reportColumns(s)
	if upd != "3" {
		t.Errorf("updating: expected upd=3, got %q", upd)
	}
	if def != "1" {
		t.Errorf("updating: expected def=1, got %q", def)
	}
	if skip != "2" {
		t.Errorf("updating: expected skip=2, got %q", skip)
	}
	if fail != "0" {
		t.Errorf("updating: expected fail=0, got %q", fail)
	}
	// Duration should be non-empty and not a dash.
	if dur == "—" || dur == "" {
		t.Errorf("updating: expected live duration, got %q", dur)
	}
}

// TestReportColumns_Success verifies that completed providers show final values.
func TestReportColumns_Success(t *testing.T) {
	lt := &LiveUpdateTable{
		states: map[string]*providerUpdateState{},
	}

	s := &providerUpdateState{
		status:        "success",
		updatedCount:  5,
		deferredCount: 0,
		skippedCount:  1,
		failedCount:   0,
		duration:      2500 * time.Millisecond,
	}
	upd, def, skip, fail, dur := lt.reportColumns(s)
	if upd != "5" || def != "0" || skip != "1" || fail != "0" || dur != "2.5s" {
		t.Errorf("success: got upd=%q def=%q skip=%q fail=%q dur=%q", upd, def, skip, fail, dur)
	}
}

// TestReportColumns_Nil verifies that a nil state returns dashes.
func TestReportColumns_Nil(t *testing.T) {
	lt := &LiveUpdateTable{
		states: map[string]*providerUpdateState{},
	}

	upd, def, skip, fail, dur := lt.reportColumns(nil)
	if upd != "—" || def != "—" || skip != "—" || fail != "—" || dur != "—" {
		t.Errorf("nil: expected all dashes, got upd=%q def=%q skip=%q fail=%q dur=%q", upd, def, skip, fail, dur)
	}
}

// TestRowStatusAndOutdated_AllStates tests status/outdated for each state.
func TestRowStatusAndOutdated_AllStates(t *testing.T) {
	lt := &LiveUpdateTable{
		states: map[string]*providerUpdateState{},
	}

	row := ScanSummaryRow{
		ProviderName:  "test",
		DisplayName:   "Test",
		OutdatedCount: 10,
		Available:     true,
	}

	tests := []struct {
		state            *providerUpdateState
		expectedStatus   string
		expectedOutdated string
	}{
		{
			state:            &providerUpdateState{status: "pending"},
			expectedStatus:   "✅ available",
			expectedOutdated: "10",
		},
		{
			state:            &providerUpdateState{status: "updating", updatedCount: 3, failedCount: 1, deferredCount: 1, skippedCount: 1},
			expectedStatus:   "🔄 updating",
			expectedOutdated: "4", // 10 - 3 - 1 - 1 - 1 = 4
		},
		{
			state:            &providerUpdateState{status: "success", updatedCount: 10},
			expectedStatus:   "✅ success",
			expectedOutdated: "0",
		},
		{
			state:            &providerUpdateState{status: "partial", updatedCount: 7, failedCount: 3},
			expectedStatus:   "📬 partial",
			expectedOutdated: "0",
		},
		{
			state:            &providerUpdateState{status: "failed"},
			expectedStatus:   "❌ failed",
			expectedOutdated: "10",
		},
		{
			state:            &providerUpdateState{status: "unavailable"},
			expectedStatus:   "⏭ unavailable",
			expectedOutdated: "-",
		},
		{
			state:            nil,
			expectedStatus:   "⏭ unavailable",
			expectedOutdated: "-",
		},
	}

	for _, tt := range tests {
		status, outdated := lt.rowStatusAndOutdated(row, tt.state)
		stateName := "nil"
		if tt.state != nil {
			stateName = tt.state.status
		}
		if status != tt.expectedStatus {
			t.Errorf("state=%s: expected status=%q, got %q", stateName, tt.expectedStatus, status)
		}
		if outdated != tt.expectedOutdated {
			t.Errorf("state=%s: expected outdated=%q, got %q", stateName, tt.expectedOutdated, outdated)
		}
	}
}

// TestRowStatusAndOutdated_Unavailable tests unavailable row.
func TestRowStatusAndOutdated_Unavailable(t *testing.T) {
	lt := &LiveUpdateTable{
		states: map[string]*providerUpdateState{},
	}

	row := ScanSummaryRow{
		ProviderName:  "test",
		Available:     false,
		OutdatedCount: 5,
	}
	s := &providerUpdateState{status: "pending"}
	status, outdated := lt.rowStatusAndOutdated(row, s)
	if status != "⏭ unavailable" || outdated != "-" {
		t.Errorf("unavailable row: got status=%q outdated=%q", status, outdated)
	}
}

// TestRowStatusAndOutdated_ScanError tests scan error row.
func TestRowStatusAndOutdated_ScanError(t *testing.T) {
	lt := &LiveUpdateTable{
		states: map[string]*providerUpdateState{},
	}

	row := ScanSummaryRow{
		ProviderName:  "test",
		Available:     true,
		OutdatedCount: 5,
		Error:         fakeErr("scan failed"),
	}
	s := &providerUpdateState{status: "pending"}
	status, outdated := lt.rowStatusAndOutdated(row, s)
	if status != "❌ scan error" || outdated != "-" {
		t.Errorf("scan error: got status=%q outdated=%q", status, outdated)
	}
}

// TestRowStatusAndOutdated_NegativeRemaining tests that remaining doesn't go below 0.
func TestRowStatusAndOutdated_NegativeRemaining(t *testing.T) {
	lt := &LiveUpdateTable{
		states: map[string]*providerUpdateState{},
	}

	row := ScanSummaryRow{
		ProviderName:  "test",
		Available:     true,
		OutdatedCount: 2,
	}
	// More updated+failed than outdated count.
	s := &providerUpdateState{status: "success", updatedCount: 5, failedCount: 1}
	_, outdated := lt.rowStatusAndOutdated(row, s)
	if outdated != "0" {
		t.Errorf("negative remaining: expected 0, got %q", outdated)
	}
}

type fakeErr string

func (e fakeErr) Error() string { return string(e) }

// TestActivePackagesFooter tests the footer line generation.
func TestActivePackagesFooter(t *testing.T) {
	lt := &LiveUpdateTable{
		rows: []ScanSummaryRow{
			{ProviderName: "brew", DisplayName: "Homebrew Formulae", Available: true},
			{ProviderName: "npm", DisplayName: "npm Global", Available: true},
			{ProviderName: "pip", DisplayName: "pip / pipx", Available: true},
		},
		states: map[string]*providerUpdateState{
			"brew": {status: "updating", currentPkg: "git"},
			"npm":  {status: "success"},
			"pip":  {status: "updating", currentPkg: ""},
		},
	}

	footer := lt.activePackagesFooter(200)
	if footer == "" {
		t.Fatal("expected non-empty footer")
	}
	if !contains(footer, "Homebrew Formulae → git") {
		t.Errorf("expected brew with pkg in footer, got %q", footer)
	}
	if !contains(footer, "pip / pipx") {
		t.Errorf("expected pip without pkg in footer, got %q", footer)
	}
	if contains(footer, "npm") {
		t.Errorf("npm is not updating, should not be in footer, got %q", footer)
	}
}

// TestActivePackagesFooter_Empty tests no footer when nothing is updating.
func TestActivePackagesFooter_Empty(t *testing.T) {
	lt := &LiveUpdateTable{
		rows: []ScanSummaryRow{
			{ProviderName: "brew", DisplayName: "Homebrew", Available: true},
		},
		states: map[string]*providerUpdateState{
			"brew": {status: "success"},
		},
	}

	footer := lt.activePackagesFooter(200)
	if footer != "" {
		t.Errorf("expected empty footer, got %q", footer)
	}
}

// TestActivePackagesFooter_Truncation tests truncation for narrow terminals.
func TestActivePackagesFooter_Truncation(t *testing.T) {
	lt := &LiveUpdateTable{
		rows: []ScanSummaryRow{
			{ProviderName: "brew", DisplayName: "Homebrew Formulae", Available: true},
		},
		states: map[string]*providerUpdateState{
			"brew": {status: "updating", currentPkg: "very-long-package-name"},
		},
	}

	footer := lt.activePackagesFooter(25)
	// Footer includes the \n prefix, so actual content starts at index 1.
	content := footer[1:] // strip leading \n
	if len(content) > 25 {
		t.Errorf("footer should be truncated to 25 chars, got %d: %q", len(content), content)
	}
	if !contains(content, "...") {
		t.Errorf("truncated footer should end with ..., got %q", content)
	}
}

// TestDisplayNameFor tests display name lookup.
func TestDisplayNameFor(t *testing.T) {
	lt := &LiveUpdateTable{
		rows: []ScanSummaryRow{
			{ProviderName: "brew", DisplayName: "Homebrew Formulae"},
			{ProviderName: "npm", DisplayName: "npm Global"},
		},
	}

	if got := lt.displayNameFor("brew"); got != "Homebrew Formulae" {
		t.Errorf("expected 'Homebrew Formulae', got %q", got)
	}
	if got := lt.displayNameFor("unknown"); got != "unknown" {
		t.Errorf("expected 'unknown' for missing provider, got %q", got)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
