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

// TestFormatUpdatingPackages tests the in-progress Packages column rendering.
func TestFormatUpdatingPackages(t *testing.T) {
	tests := []struct {
		name      string
		completed []string
		all       []string
		maxWidth  int
		want      []string
	}{
		{
			name:      "merged on one line",
			completed: []string{"chatgpt", "codex"},
			all:       []string{"chatgpt", "codex", "codex-app", "cursor"},
			maxWidth:  200,
			want:      []string{"chatgpt, codex | Remaining: codex-app, cursor"},
		},
		{
			name:      "split when too narrow to merge",
			completed: []string{"chatgpt", "codex"},
			all:       []string{"chatgpt", "codex", "codex-app", "cursor"},
			maxWidth:  20,
			want:      []string{"chatgpt, codex", "Remaining: codex-app", "cursor"},
		},
		{
			name:      "nothing completed yet",
			completed: nil,
			all:       []string{"a", "b", "c"},
			maxWidth:  200,
			want:      []string{"Remaining: a, b, c"},
		},
		{
			name:      "everything completed",
			completed: []string{"a", "b"},
			all:       []string{"a", "b"},
			maxWidth:  200,
			want:      []string{"a, b"},
		},
		{
			name:      "both empty",
			completed: nil,
			all:       nil,
			maxWidth:  200,
			want:      []string{"-"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatUpdatingPackages(tt.completed, tt.all, tt.maxWidth)
			if len(got) != len(tt.want) {
				t.Fatalf("expected %d lines, got %d: %#v", len(tt.want), len(got), got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("line %d: expected %q, got %q", i, tt.want[i], got[i])
				}
			}
		})
	}
}

// TestBuildUpdateTableRows_NonGrouped_Updating verifies that a provider whose
// state is "updating" produces a row flagged for the "done | Remaining" format
// with packages drawn from state.packages and allPackages from scan-time r.Packages.
func TestBuildUpdateTableRows_NonGrouped_Updating(t *testing.T) {
	lt := &LiveUpdateTable{
		rows: []ScanSummaryRow{
			{
				ProviderName:  "brew",
				DisplayName:   "Homebrew Casks",
				OutdatedCount: 4,
				Packages:      []string{"chatgpt", "codex", "cursor", "discord"},
				Available:     true,
			},
		},
		states: map[string]*providerUpdateState{
			"brew": {
				status:       "updating",
				updatedCount: 1,
				startTime:    time.Now(),
				packages:     []string{"chatgpt"},
			},
		},
	}

	rows := lt.buildUpdateTableRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	ir := rows[0]
	if !ir.updating {
		t.Errorf("expected updating=true, got false")
	}
	if len(ir.packages) != 1 || ir.packages[0] != "chatgpt" {
		t.Errorf("expected packages=[chatgpt], got %#v", ir.packages)
	}
	wantAll := []string{"chatgpt", "codex", "cursor", "discord"}
	if len(ir.allPackages) != len(wantAll) {
		t.Fatalf("expected allPackages len %d, got %d (%#v)", len(wantAll), len(ir.allPackages), ir.allPackages)
	}
	for i, p := range wantAll {
		if ir.allPackages[i] != p {
			t.Errorf("allPackages[%d]: want %q, got %q", i, p, ir.allPackages[i])
		}
	}
}

// TestBuildUpdateTableRows_NonGrouped_Completed verifies that a successfully
// completed provider does NOT use the updating format and reuses state.packages
// as the final list.
func TestBuildUpdateTableRows_NonGrouped_Completed(t *testing.T) {
	lt := &LiveUpdateTable{
		rows: []ScanSummaryRow{
			{
				ProviderName:  "brew",
				DisplayName:   "Homebrew Casks",
				OutdatedCount: 2,
				Packages:      []string{"chatgpt", "codex"},
				Available:     true,
			},
		},
		states: map[string]*providerUpdateState{
			"brew": {
				status:       "success",
				updatedCount: 2,
				packages:     []string{"chatgpt", "codex"},
			},
		},
	}

	rows := lt.buildUpdateTableRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	ir := rows[0]
	if ir.updating {
		t.Errorf("expected updating=false, got true")
	}
	if len(ir.allPackages) != 0 {
		t.Errorf("expected allPackages=nil for non-updating, got %#v", ir.allPackages)
	}
}

// TestBuildUpdateTableRows_NonGrouped_Pending verifies that a pending provider
// uses r.Packages (scan-time list) as the package list.
func TestBuildUpdateTableRows_NonGrouped_Pending(t *testing.T) {
	lt := &LiveUpdateTable{
		rows: []ScanSummaryRow{
			{
				ProviderName:  "brew",
				DisplayName:   "Homebrew Casks",
				OutdatedCount: 2,
				Packages:      []string{"a", "b"},
				Available:     true,
			},
		},
		states: map[string]*providerUpdateState{
			"brew": {status: "pending"},
		},
	}

	rows := lt.buildUpdateTableRows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].updating {
		t.Errorf("expected updating=false for pending state")
	}
	if len(rows[0].packages) != 2 || rows[0].packages[0] != "a" {
		t.Errorf("expected packages=[a,b] from r.Packages, got %#v", rows[0].packages)
	}
}

// TestBuildUpdateTableRows_Grouped verifies that providers with PackageGroups
// produce a parent row plus one sub-row per group, with the sub-rows carrying
// the group's package list and never using the updating format.
func TestBuildUpdateTableRows_Grouped(t *testing.T) {
	lt := &LiveUpdateTable{
		rows: []ScanSummaryRow{
			{
				ProviderName:  "ext",
				DisplayName:   "Code Editor Extensions",
				OutdatedCount: 2,
				Available:     true,
				PackageGroups: map[string][]string{
					"code":   {"openai.chatgpt"},
					"cursor": {"ms-python.python"},
				},
			},
		},
		states: map[string]*providerUpdateState{
			"ext": {status: "updating", startTime: time.Now()},
		},
	}

	rows := lt.buildUpdateTableRows()
	// Expect 1 parent + 2 sub-rows.
	if len(rows) != 3 {
		t.Fatalf("expected 3 rows (parent + 2 sub), got %d: %#v", len(rows), rows)
	}
	if rows[0].provider != "Code Editor Extensions" {
		t.Errorf("expected parent row first, got %q", rows[0].provider)
	}
	for i := 1; i < len(rows); i++ {
		if rows[i].updating {
			t.Errorf("sub-row %d should not be marked updating", i)
		}
		if len(rows[i].packages) == 0 {
			t.Errorf("sub-row %d expected packages, got empty", i)
		}
	}
}

// TestMaxPackageWidth_NarrowFloor verifies that very narrow terminals floor at 10.
func TestMaxPackageWidth_NarrowFloor(t *testing.T) {
	rows := []updateTableRow{
		{provider: "p", status: "s", outdated: "1", upd: "0", def: "0", skip: "0", fail: "0", dur: "1s"},
	}
	if got := maxPackageWidth(rows, 10); got != 10 {
		t.Errorf("expected floor of 10, got %d", got)
	}
}

// TestMaxPackageWidth_WideTerm verifies leftover width with a wide terminal.
func TestMaxPackageWidth_WideTerm(t *testing.T) {
	rows := []updateTableRow{
		{provider: "Homebrew Casks", status: "🔄 updating", outdated: "6", upd: "2", def: "0", skip: "0", fail: "0", dur: "36.9s"},
	}
	// Wide terminal — leftover should be positive and well above the floor.
	got := maxPackageWidth(rows, 200)
	if got < 50 {
		t.Errorf("expected wide leftover, got %d", got)
	}
}

// TestBuildTableData_Updating verifies that an updating row produces a Packages
// cell rendered as "done | Remaining: rest" via formatUpdatingPackages.
func TestBuildTableData_Updating(t *testing.T) {
	rows := []updateTableRow{
		{
			provider: "Homebrew Casks", status: "🔄 updating", outdated: "4",
			upd: "2", def: "0", skip: "0", fail: "0", dur: "1s",
			packages: []string{"chatgpt", "codex"}, updating: true,
			allPackages: []string{"chatgpt", "codex", "codex-app", "cursor"},
		},
	}
	data := buildTableData(rows, 200)
	// data[0] is header, data[1] is the only data row.
	if len(data) != 2 {
		t.Fatalf("expected 2 rows (header + data), got %d", len(data))
	}
	pkgCell := data[1][8]
	want := "chatgpt, codex | Remaining: codex-app, cursor"
	if pkgCell != want {
		t.Errorf("expected Packages cell %q, got %q", want, pkgCell)
	}
}

// TestBuildTableData_NonUpdating verifies WrapPackages is used for non-updating rows.
func TestBuildTableData_NonUpdating(t *testing.T) {
	rows := []updateTableRow{
		{
			provider: "Homebrew Casks", status: "✅ success", outdated: "0",
			upd: "2", def: "0", skip: "0", fail: "0", dur: "1s",
			packages: []string{"chatgpt", "codex"},
		},
	}
	data := buildTableData(rows, 200)
	pkgCell := data[1][8]
	if pkgCell != "chatgpt, codex" {
		t.Errorf("expected Packages cell %q, got %q", "chatgpt, codex", pkgCell)
	}
}

// TestBuildTableData_WrappedContinuation verifies that long package lists wrap
// onto continuation rows (which use blank cells for non-package columns).
func TestBuildTableData_WrappedContinuation(t *testing.T) {
	rows := []updateTableRow{
		{
			provider: "brew", status: "✅ success", outdated: "0",
			upd: "3", def: "0", skip: "0", fail: "0", dur: "1s",
			packages: []string{"alpha-package", "beta-package", "gamma-package"},
		},
	}
	// Narrow Packages column forces a wrap.
	data := buildTableData(rows, 15)
	if len(data) < 3 {
		t.Fatalf("expected header + data + continuation, got %d rows", len(data))
	}
	// Continuation row must have blank non-package cells.
	cont := data[2]
	for i := 0; i < 8; i++ {
		if cont[i] != "" {
			t.Errorf("continuation row col %d should be empty, got %q", i, cont[i])
		}
	}
	if cont[8] == "" {
		t.Errorf("continuation row should carry a packages chunk, got empty")
	}
}

// TestRenderTableContent_UpdatingProvider verifies the end-to-end rendered
// content includes the "Remaining:" marker for a provider mid-update.
func TestRenderTableContent_UpdatingProvider(t *testing.T) {
	lt := &LiveUpdateTable{
		rows: []ScanSummaryRow{
			{
				ProviderName:  "brew",
				DisplayName:   "Homebrew Casks",
				OutdatedCount: 4,
				Packages:      []string{"chatgpt", "codex", "codex-app", "cursor"},
				Available:     true,
			},
		},
		states: map[string]*providerUpdateState{
			"brew": {
				status:       "updating",
				updatedCount: 2,
				startTime:    time.Now(),
				packages:     []string{"chatgpt", "codex"},
				currentPkg:   "codex-app",
			},
		},
	}

	content := lt.renderTableContent(200)
	if content == "" {
		t.Fatal("expected non-empty rendered content")
	}
	if !contains(content, "Remaining: codex-app") {
		t.Errorf("expected 'Remaining: codex-app' in content, got:\n%s", content)
	}
	if !contains(content, "chatgpt, codex") {
		t.Errorf("expected 'chatgpt, codex' (done list) in content, got:\n%s", content)
	}
	// Footer should also show the actively updating package.
	if !contains(content, "Homebrew Casks → codex-app") {
		t.Errorf("expected active footer in content, got:\n%s", content)
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
