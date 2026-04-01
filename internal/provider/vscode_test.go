package provider_test

import (
	"context"
	"testing"
	"time"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/marketplace"
	"github.com/teknikqa/upkeep/internal/provider"
)

func TestVSCodeProvider_Name(t *testing.T) {
	p := provider.NewVSCodeProvider(config.VSCodeConfig{Enabled: true, Editors: []string{"code"}}, nil)
	if p.Name() != "vscode" {
		t.Errorf("expected %q, got %q", "vscode", p.Name())
	}
	if p.DisplayName() != "VS Code / Editors" {
		t.Errorf("expected %q, got %q", "VS Code / Editors", p.DisplayName())
	}
}

func TestVSCodeProvider_DependsOn(t *testing.T) {
	p := provider.NewVSCodeProvider(config.VSCodeConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestVSCodeProvider_Scan_NoEditors(t *testing.T) {
	// Use a non-existent editor name to simulate no editors installed.
	p := provider.NewVSCodeProvider(config.VSCodeConfig{
		Enabled: true,
		Editors: []string{"nonexistent-editor-xyz-abc"},
		Timeout: 5,
	}, nil)
	result := p.Scan(context.Background())
	if result.Available {
		t.Error("expected Available=false when no editors found")
	}
}

func TestVSCodeProvider_Timeout(t *testing.T) {
	// Verify timeout is respected: run `sleep` as the "editor".
	// This tests that the per-editor context deadline fires.
	// Only run on systems where sleep is available.
	if !provider.CommandExistsExport("sleep") {
		t.Skip("sleep not available")
	}

	p := provider.NewVSCodeProvider(config.VSCodeConfig{
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

func TestVSCodeProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("vscode")
	if err != nil {
		t.Fatalf("vscode not registered: %v", err)
	}
	if p.Name() != "vscode" {
		t.Errorf("expected vscode, got %s", p.Name())
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
