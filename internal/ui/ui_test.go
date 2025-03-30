package ui_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/teknikqa/upkeep/internal/provider"
	"github.com/teknikqa/upkeep/internal/ui"
)

func TestRenderScanSummaryTable_NoRows(t *testing.T) {
	// Should not panic with empty rows.
	ui.RenderScanSummaryTable(nil)
	ui.RenderScanSummaryTable([]ui.ScanSummaryRow{})
}

func TestRenderScanSummaryTable_WithRows(t *testing.T) {
	// Just ensure it doesn't panic.
	rows := []ui.ScanSummaryRow{
		{
			ProviderName:  "brew",
			DisplayName:   "Homebrew Formulae",
			OutdatedCount: 3,
			Packages:      []string{"git", "jq", "ripgrep"},
			Available:     true,
		},
		{
			ProviderName: "npm",
			DisplayName:  "npm (global)",
			Available:    false,
		},
	}
	ui.RenderScanSummaryTable(rows)
}

func TestRenderFinalReport_NoPanic(t *testing.T) {
	rows := []ui.UpdateSummaryRow{
		{
			ProviderName: "brew",
			DisplayName:  "Homebrew Formulae",
			Updated:      3,
			Duration:     45 * time.Second,
			Status:       "success",
		},
		{
			ProviderName: "brew-cask",
			DisplayName:  "Homebrew Casks",
			Updated:      2,
			Deferred:     1,
			Duration:     30 * time.Second,
			Status:       "partial",
		},
	}
	ui.RenderFinalReport(rows, 75*time.Second)
}

func TestStatusLine_Output(t *testing.T) {
	var buf bytes.Buffer
	ui.StatusLine(&buf, "Homebrew Formulae", "success", 3, 0, 0, 45*time.Second)
	out := buf.String()
	if !strings.Contains(out, "Homebrew Formulae") {
		t.Errorf("expected output to contain 'Homebrew Formulae', got %q", out)
	}
	if !strings.Contains(out, "updated=3") {
		t.Errorf("expected 'updated=3' in output, got %q", out)
	}
}

func TestConfirm_YesFlag(t *testing.T) {
	// With yesFlag=true, should always return true without prompting.
	result := ui.Confirm("Update 3 packages?", true)
	if !result {
		t.Error("expected Confirm to return true when yesFlag=true")
	}
}

func TestScanSummaryRowsFromResults(t *testing.T) {
	results := map[string]provider.ScanResult{
		"brew": {
			Available: true,
			Outdated: []provider.OutdatedItem{
				{Name: "git", CurrentVersion: "2.39.0", LatestVersion: "2.40.0"},
				{Name: "jq", CurrentVersion: "1.6", LatestVersion: "1.7"},
			},
		},
		"npm": {
			Available: false,
		},
	}
	displayNames := map[string]string{
		"brew": "Homebrew Formulae",
		"npm":  "npm (global)",
	}

	rows := ui.ScanSummaryRowsFromResults(results, displayNames)
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// Find brew row.
	var brewRow *ui.ScanSummaryRow
	for i := range rows {
		if rows[i].ProviderName == "brew" {
			brewRow = &rows[i]
			break
		}
	}
	if brewRow == nil {
		t.Fatal("expected brew row in results")
	}
	if brewRow.OutdatedCount != 2 {
		t.Errorf("expected OutdatedCount=2, got %d", brewRow.OutdatedCount)
	}
	if brewRow.DisplayName != "Homebrew Formulae" {
		t.Errorf("expected display name 'Homebrew Formulae', got %q", brewRow.DisplayName)
	}
}
