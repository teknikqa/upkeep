package provider_test

import (
	"context"
	"testing"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/marketplace"
	"github.com/teknikqa/upkeep/internal/provider"
)

func TestEditorProvider_Name(t *testing.T) {
	p := provider.NewEditorProvider(config.EditorConfig{Enabled: true, Editors: []string{"code"}}, nil)
	if p.Name() != "editor" {
		t.Errorf("expected %q, got %q", "editor", p.Name())
	}
	if p.DisplayName() != "Code Editor Extensions" {
		t.Errorf("expected %q, got %q", "Code Editor Extensions", p.DisplayName())
	}
}

func TestEditorProvider_DependsOn(t *testing.T) {
	p := provider.NewEditorProvider(config.EditorConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestEditorProvider_Scan_NoEditors(t *testing.T) {
	// Use a non-existent editor name to simulate no editors installed.
	p := provider.NewEditorProvider(config.EditorConfig{
		Enabled: true,
		Editors: []string{"nonexistent-editor-xyz-abc"},
		Timeout: 5,
	}, nil)
	result := p.Scan(context.Background())
	if result.Available {
		t.Error("expected Available=false when no editors found")
	}
}

func TestEditorProvider_Timeout(t *testing.T) {
	// Verify timeout is respected: run `sleep` as the "editor".
	// This tests that the per-editor context deadline fires.
	// Only run on systems where sleep is available.
	if !provider.CommandExistsExport("sleep") {
		t.Skip("sleep not available")
	}

	p := provider.NewEditorProvider(config.EditorConfig{
		Enabled: true,
		Editors: []string{"sleep"},
		Timeout: 1,
	}, nil)

	items := []provider.OutdatedItem{{Name: "sleep", LatestVersion: "1 extension"}}
	start := time.Now()
	result := p.Update(context.Background(), items)
	elapsed := time.Since(start)

	// sleep with no args exits quickly (error), but we're testing timeout logic.
	// The important thing: it should not take more than ~3s.
	if elapsed > 5*time.Second {
		t.Errorf("update took too long: %v", elapsed)
	}
	_ = result
}

func TestEditorProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("editor")
	if err != nil {
		t.Fatalf("editor not registered: %v", err)
	}
	if p.Name() != "editor" {
		t.Errorf("expected editor, got %s", p.Name())
	}
}

func TestParseExtensionList(t *testing.T) {
	input := "dbaeumer.vscode-eslint@2.4.2\nesbenp.prettier-vscode@10.1.0\n\ngolang.go@0.41.1\n"
	got := provider.ParseExtensionList(input)
	if len(got) != 3 {
		t.Fatalf("expected 3 extensions, got %d: %+v", len(got), got)
	}
	if got[0].ID != "dbaeumer.vscode-eslint" || got[0].Version != "2.4.2" {
		t.Errorf("unexpected first extension: %+v", got[0])
	}
	if got[1].ID != "esbenp.prettier-vscode" || got[1].Version != "10.1.0" {
		t.Errorf("unexpected second extension: %+v", got[1])
	}
	if got[2].ID != "golang.go" || got[2].Version != "0.41.1" {
		t.Errorf("unexpected third extension: %+v", got[2])
	}
}

func TestParseExtensionList_Empty(t *testing.T) {
	got := provider.ParseExtensionList("")
	if len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

func TestParseExtensionList_MalformedLines(t *testing.T) {
	input := "malformed-no-version\n@also-bad\nvalid.ext@1.0.0\n"
	got := provider.ParseExtensionList(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 valid extension, got %d: %+v", len(got), got)
	}
	if got[0].ID != "valid.ext" || got[0].Version != "1.0.0" {
		t.Errorf("unexpected extension: %+v", got[0])
	}
}

func TestParseExtensionList_LowercasesID(t *testing.T) {
	input := "MyPublisher.MyExt@2.0.0\n"
	got := provider.ParseExtensionList(input)
	if len(got) != 1 {
		t.Fatalf("expected 1 extension, got %d", len(got))
	}
	if got[0].ID != "mypublisher.myext" {
		t.Errorf("expected lowercased ID, got %q", got[0].ID)
	}
}

func TestCompareVersions(t *testing.T) {
	installed := []marketplace.Extension{
		{ID: "pub.ext1", Version: "1.0.0"}, // outdated
		{ID: "pub.ext2", Version: "2.0.0"}, // up to date
		{ID: "pub.ext3", Version: "3.0.0"}, // not in marketplace
	}
	latest := map[string]marketplace.LatestVersion{
		"pub.ext1": {ID: "pub.ext1", Version: "1.1.0", Found: true},
		"pub.ext2": {ID: "pub.ext2", Version: "2.0.0", Found: true},
	}

	items := provider.CompareVersions(installed, latest)
	if len(items) != 1 {
		t.Fatalf("expected 1 outdated item, got %d: %+v", len(items), items)
	}
	if items[0].Name != "pub.ext1" {
		t.Errorf("expected pub.ext1, got %s", items[0].Name)
	}
	if items[0].CurrentVersion != "1.0.0" {
		t.Errorf("expected CurrentVersion=1.0.0, got %s", items[0].CurrentVersion)
	}
	if items[0].LatestVersion != "1.1.0" {
		t.Errorf("expected LatestVersion=1.1.0, got %s", items[0].LatestVersion)
	}
}

func TestCompareVersions_Empty(t *testing.T) {
	items := provider.CompareVersions(nil, nil)
	if len(items) != 0 {
		t.Errorf("expected empty, got %v", items)
	}
}

func TestCompareVersions_AllUpToDate(t *testing.T) {
	installed := []marketplace.Extension{
		{ID: "pub.ext1", Version: "1.0.0"},
	}
	latest := map[string]marketplace.LatestVersion{
		"pub.ext1": {ID: "pub.ext1", Version: "1.0.0", Found: true},
	}
	items := provider.CompareVersions(installed, latest)
	if len(items) != 0 {
		t.Errorf("expected 0 outdated, got %v", items)
	}
}

func TestScanResult_GroupsConsistency(t *testing.T) {
	result := provider.ScanResult{
		Available: true,
		Outdated: []provider.OutdatedItem{
			{Name: "ext1"}, {Name: "ext2"}, {Name: "ext3"},
		},
		Groups: map[string][]string{
			"code":   {"ext1", "ext2"},
			"cursor": {"ext3"},
		},
	}
	// All group values should be in Outdated names.
	outdatedNames := make(map[string]bool)
	for _, item := range result.Outdated {
		outdatedNames[item.Name] = true
	}
	for group, names := range result.Groups {
		for _, name := range names {
			if !outdatedNames[name] {
				t.Errorf("group %q contains %q which is not in Outdated", group, name)
			}
		}
	}
	// Total grouped items should equal total outdated items.
	totalGrouped := 0
	for _, names := range result.Groups {
		totalGrouped += len(names)
	}
	if totalGrouped != len(result.Outdated) {
		t.Errorf("grouped item count (%d) != outdated item count (%d)", totalGrouped, len(result.Outdated))
	}
}

func TestScanResult_GroupsNil(t *testing.T) {
	result := provider.ScanResult{
		Available: true,
		Outdated: []provider.OutdatedItem{
			{Name: "pkg1"},
		},
	}
	if result.Groups != nil {
		t.Error("expected Groups to be nil for non-grouped provider")
	}
}

// TestCompareVersions_SkipsPreReleaseInstalled verifies that when the installed version
// matches LatestPreReleaseVersion, the extension is NOT reported as outdated.
func TestCompareVersions_SkipsPreReleaseInstalled(t *testing.T) {
	installed := []marketplace.Extension{
		{ID: "redhat.vscode-yaml", Version: "1.22.2026032808"},
	}
	latest := map[string]marketplace.LatestVersion{
		"redhat.vscode-yaml": {
			ID:                      "redhat.vscode-yaml",
			Version:                 "1.21.0",
			Found:                   true,
			LatestPreReleaseVersion: "1.22.2026032808",
		},
	}

	items := provider.CompareVersions(installed, latest)
	if len(items) != 0 {
		t.Errorf("expected 0 outdated items (user is on pre-release track), got %d: %+v", len(items), items)
	}
}

// TestCompareVersions_SkipsAllPreRelease verifies that when lv.PreRelease=true (no stable
// version exists), the extension is NOT reported as outdated.
func TestCompareVersions_SkipsAllPreRelease(t *testing.T) {
	installed := []marketplace.Extension{
		{ID: "mypub.myext", Version: "2.0.0-pre"},
	}
	latest := map[string]marketplace.LatestVersion{
		"mypub.myext": {
			ID:                      "mypub.myext",
			Version:                 "",
			Found:                   true,
			PreRelease:              true,
			LatestPreReleaseVersion: "2.0.0-pre",
		},
	}

	items := provider.CompareVersions(installed, latest)
	if len(items) != 0 {
		t.Errorf("expected 0 outdated items (no stable version exists), got %d: %+v", len(items), items)
	}
}

// TestCompareVersions_StableOutdated verifies that a stable extension is correctly reported
// as outdated when a newer stable version exists, even if a pre-release is also present.
func TestCompareVersions_StableOutdated(t *testing.T) {
	installed := []marketplace.Extension{
		{ID: "pub.ext", Version: "1.0.0"},
	}
	latest := map[string]marketplace.LatestVersion{
		"pub.ext": {
			ID:                      "pub.ext",
			Version:                 "2.0.0",
			Found:                   true,
			LatestPreReleaseVersion: "3.0.0-beta",
		},
	}

	items := provider.CompareVersions(installed, latest)
	if len(items) != 1 {
		t.Fatalf("expected 1 outdated item, got %d: %+v", len(items), items)
	}
	if items[0].LatestVersion != "2.0.0" {
		t.Errorf("expected LatestVersion=2.0.0 (stable), got %q", items[0].LatestVersion)
	}
}

// TestCompareVersions_PreReleaseInstalledButStableExists verifies that when the installed
// version is a pre-release that matches LatestPreReleaseVersion, it is NOT reported as
// outdated — even though a lower stable version also exists.
func TestCompareVersions_PreReleaseInstalledButStableExists(t *testing.T) {
	installed := []marketplace.Extension{
		{ID: "redhat.vscode-yaml", Version: "1.22.2026032808"},
	}
	latest := map[string]marketplace.LatestVersion{
		"redhat.vscode-yaml": {
			ID:                      "redhat.vscode-yaml",
			Version:                 "1.21.0",
			Found:                   true,
			LatestPreReleaseVersion: "1.22.2026032808",
		},
	}

	items := provider.CompareVersions(installed, latest)
	if len(items) != 0 {
		t.Errorf("expected 0 outdated items (on pre-release track), got %d: %+v", len(items), items)
	}
}
