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
	// Just ensure it doesn't panic — including grouped rows.
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
		{
			ProviderName:  "editor",
			DisplayName:   "Code Editor Extensions",
			OutdatedCount: 5,
			Packages:      []string{"ext1", "ext2", "ext3", "ext4", "ext5"},
			PackageGroups: map[string][]string{
				"code":   {"ext1", "ext2", "ext3"},
				"cursor": {"ext4", "ext5"},
			},
			Available: true,
		},
	}
	ui.RenderScanSummaryTable(rows)
}

func TestStatusLine_Output(t *testing.T) {
	var buf bytes.Buffer
	ui.StatusLine(&buf, "Homebrew Formulae", "success", 3, 0, 0, 0, 45*time.Second)
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
	if brewRow.PackageGroups != nil {
		t.Error("expected PackageGroups to be nil for non-grouped provider")
	}
}

func TestFormatGroupedPackageList(t *testing.T) {
	tests := []struct {
		name   string
		groups map[string][]string
		want   string
	}{
		{
			name:   "multiple groups",
			groups: map[string][]string{"code": {"ext1", "ext2"}, "cursor": {"ext3"}},
			want:   "code: ext1, ext2; cursor: ext3",
		},
		{
			name:   "single group",
			groups: map[string][]string{"code": {"ext1"}},
			want:   "code: ext1",
		},
		{
			name:   "nil groups",
			groups: nil,
			want:   "-",
		},
		{
			name:   "empty groups map",
			groups: map[string][]string{},
			want:   "-",
		},
		{
			name:   "group with empty slice",
			groups: map[string][]string{"code": {}},
			want:   "-",
		},
		{
			name:   "sorted group keys",
			groups: map[string][]string{"windsurf": {"ext3"}, "agy": {"ext1"}, "code": {"ext2"}},
			want:   "agy: ext1; code: ext2; windsurf: ext3",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ui.FormatGroupedPackageList(tt.groups)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestScanSummaryRowsFromResults_WithGroups(t *testing.T) {
	results := map[string]provider.ScanResult{
		"editor": {
			Available: true,
			Outdated: []provider.OutdatedItem{
				{Name: "ext1"}, {Name: "ext2"}, {Name: "ext3"},
			},
			Groups: map[string][]string{
				"code":   {"ext1", "ext2"},
				"cursor": {"ext3"},
			},
		},
	}
	displayNames := map[string]string{"editor": "Code Editor Extensions"}

	rows := ui.ScanSummaryRowsFromResults(results, displayNames)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	row := rows[0]
	if row.PackageGroups == nil {
		t.Fatal("expected PackageGroups to be non-nil")
	}
	if len(row.PackageGroups["code"]) != 2 {
		t.Errorf("expected 2 packages in code group, got %d", len(row.PackageGroups["code"]))
	}
	if len(row.PackageGroups["cursor"]) != 1 {
		t.Errorf("expected 1 package in cursor group, got %d", len(row.PackageGroups["cursor"]))
	}
}

func TestGroupSubRows(t *testing.T) {
	tests := []struct {
		name   string
		groups map[string][]string
		want   []ui.GroupSubRow
	}{
		{
			name:   "nil groups",
			groups: nil,
			want:   nil,
		},
		{
			name:   "empty groups",
			groups: map[string][]string{},
			want:   nil,
		},
		{
			name:   "single group",
			groups: map[string][]string{"code": {"ext1", "ext2"}},
			want: []ui.GroupSubRow{
				{Label: "  └ code", Count: 2, Packages: "ext1, ext2"},
			},
		},
		{
			name: "multiple groups sorted",
			groups: map[string][]string{
				"cursor": {"ext3"},
				"agy":    {"ext1"},
				"code":   {"ext2", "ext4"},
			},
			want: []ui.GroupSubRow{
				{Label: "  ├ agy", Count: 1, Packages: "ext1"},
				{Label: "  ├ code", Count: 2, Packages: "ext2, ext4"},
				{Label: "  └ cursor", Count: 1, Packages: "ext3"},
			},
		},
		{
			name:   "group with empty slice skipped",
			groups: map[string][]string{"code": {"ext1"}, "empty": {}},
			want: []ui.GroupSubRow{
				{Label: "  └ code", Count: 1, Packages: "ext1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ui.GroupSubRows(tt.groups)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d rows, want %d", len(got), len(tt.want))
			}
			for i, want := range tt.want {
				if got[i].Label != want.Label {
					t.Errorf("row %d: label got %q, want %q", i, got[i].Label, want.Label)
				}
				if got[i].Count != want.Count {
					t.Errorf("row %d: count got %d, want %d", i, got[i].Count, want.Count)
				}
				if got[i].Packages != want.Packages {
					t.Errorf("row %d: packages got %q, want %q", i, got[i].Packages, want.Packages)
				}
			}
		})
	}
}

func TestWrapPackages(t *testing.T) {
	tests := []struct {
		name     string
		pkgs     []string
		maxWidth int
		want     []string
	}{
		{
			name:     "empty list returns dash",
			pkgs:     nil,
			maxWidth: 40,
			want:     []string{"-"},
		},
		{
			name:     "fits on one line",
			pkgs:     []string{"git", "jq", "ripgrep"},
			maxWidth: 40,
			want:     []string{"git, jq, ripgrep"},
		},
		{
			name:     "wraps to two lines",
			pkgs:     []string{"alpha", "bravo", "charlie", "delta", "echo"},
			maxWidth: 20,
			want:     []string{"alpha, bravo", "charlie, delta, echo"},
		},
		{
			name:     "single package per line when narrow",
			pkgs:     []string{"longpkgname1", "longpkgname2", "longpkgname3"},
			maxWidth: 12,
			want:     []string{"longpkgname1", "longpkgname2", "longpkgname3"},
		},
		{
			name:     "single package always on first line even if wider than maxWidth",
			pkgs:     []string{"very-long-package-name"},
			maxWidth: 5,
			want:     []string{"very-long-package-name"},
		},
		{
			name: "many packages wraps correctly",
			pkgs: []string{
				"bmewburn.vscode-intelephense-client",
				"github.copilot-chat",
				"ms-python.debugpy",
				"ms-python.python",
				"ms-python.vscode-python-envs",
				"ms-vscode-remote.remote-containers",
				"ms-vscode.makefile-tools",
				"openai.chatgpt",
				"redhat.vscode-xml",
				"redhat.vscode-yaml",
			},
			maxWidth: 60,
			want: []string{
				"bmewburn.vscode-intelephense-client, github.copilot-chat",
				"ms-python.debugpy, ms-python.python",
				"ms-python.vscode-python-envs",
				"ms-vscode-remote.remote-containers, ms-vscode.makefile-tools",
				"openai.chatgpt, redhat.vscode-xml, redhat.vscode-yaml",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ui.WrapPackages(tt.pkgs, tt.maxWidth)
			if len(got) != len(tt.want) {
				t.Fatalf("got %d lines, want %d\ngot:  %v\nwant: %v", len(got), len(tt.want), got, tt.want)
			}
			for i, want := range tt.want {
				if got[i] != want {
					t.Errorf("line %d: got %q, want %q", i, got[i], want)
				}
			}
		})
	}
}
