package provider_test

import (
	"context"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

const sampleMasOutdated = `{"adamID":1352778147,"name":"Bitwarden","version":"2026.7.0","newVersion":"2026.8.0"}
{"adamID":462058435,"name":"Microsoft Excel","version":"16.111.3","newVersion":"16.111.4"}
`

func TestMasProvider_Name(t *testing.T) {
	p := provider.NewMasProvider(config.MasConfig{Enabled: true}, nil)
	if p.Name() != "mas" {
		t.Errorf("expected %q, got %q", "mas", p.Name())
	}
	if p.DisplayName() != "Mac App Store Apps" {
		t.Errorf("expected %q, got %q", "Mac App Store Apps", p.DisplayName())
	}
}

func TestMasProvider_DependsOn(t *testing.T) {
	p := provider.NewMasProvider(config.MasConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestParseMasOutdated(t *testing.T) {
	items := provider.ParseMasOutdated(sampleMasOutdated)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	byName := make(map[string]provider.OutdatedItem, len(items))
	for _, item := range items {
		byName[item.Name] = item
	}

	bitwarden := byName["Bitwarden"]
	if bitwarden.AppID != "1352778147" {
		t.Errorf("Bitwarden AppID: expected 1352778147, got %q", bitwarden.AppID)
	}
	if bitwarden.CurrentVersion != "2026.7.0" {
		t.Errorf("Bitwarden current: expected 2026.7.0, got %q", bitwarden.CurrentVersion)
	}
	if bitwarden.LatestVersion != "2026.8.0" {
		t.Errorf("Bitwarden latest: expected 2026.8.0, got %q", bitwarden.LatestVersion)
	}

	excel := byName["Microsoft Excel"]
	if excel.AppID != "462058435" {
		t.Errorf("Excel AppID: expected 462058435, got %q", excel.AppID)
	}
}

func TestParseMasOutdated_Empty(t *testing.T) {
	if items := provider.ParseMasOutdated(""); len(items) != 0 {
		t.Errorf("expected 0 items for empty output, got %d", len(items))
	}
}

func TestParseMasOutdated_SkipsBlankLines(t *testing.T) {
	input := `{"adamID":1352778147,"name":"Bitwarden","version":"2026.7.0","newVersion":"2026.8.0"}

{"adamID":462058435,"name":"Microsoft Excel","version":"16.111.3","newVersion":"16.111.4"}
`
	items := provider.ParseMasOutdated(input)
	if len(items) != 2 {
		t.Fatalf("expected 2 items with the blank line skipped, got %d", len(items))
	}
}

func TestParseMasOutdated_SkipsMalformedLines(t *testing.T) {
	// mas outdated emits one JSON object per line (not a JSON array), so a
	// single malformed line shouldn't take down the whole parse.
	input := `{"adamID":1352778147,"name":"Bitwarden","version":"2026.7.0","newVersion":"2026.8.0"}
not valid json
{"adamID":462058435,"name":"Microsoft Excel","version":"16.111.3","newVersion":"16.111.4"}
`
	items := provider.ParseMasOutdated(input)
	if len(items) != 2 {
		t.Fatalf("expected 2 valid items with the malformed line skipped, got %d", len(items))
	}
}

func TestMasProvider_Scan_NoMas(t *testing.T) {
	if provider.CommandExistsExport("mas") {
		t.Skip("mas is installed; skipping unavailability test")
	}
	p := provider.NewMasProvider(config.MasConfig{Enabled: true}, nil)
	result := p.Scan(context.Background())
	if result.Available {
		t.Error("expected Available=false when mas not installed")
	}
}

func TestMasProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("mas")
	if err != nil {
		t.Fatalf("mas not registered: %v", err)
	}
	if p.Name() != "mas" {
		t.Errorf("expected mas, got %s", p.Name())
	}
}

func TestMasProvider_Update_Empty(t *testing.T) {
	p := provider.NewMasProvider(config.MasConfig{Enabled: true}, nil)
	result := p.Update(context.Background(), nil)
	if len(result.Updated) != 0 || len(result.Failed) != 0 {
		t.Errorf("expected empty result for nil items, got updated=%v failed=%v", result.Updated, result.Failed)
	}
}

func TestMasProvider_Update_ItemsAccountedFor(t *testing.T) {
	p := provider.NewMasProvider(config.MasConfig{Enabled: true}, nil)
	// Fake app IDs that don't correspond to real installed apps, so the
	// batch and per-item fallback both fail — but every item must still be
	// accounted for. This also exercises the sudo -v pre-cache: with no tty
	// and no cached ticket it fails fast rather than hanging (verified
	// manually before adding this test).
	items := []provider.OutdatedItem{
		{Name: "mas-nonexistent-app-aaa", AppID: "1"},
		{Name: "mas-nonexistent-app-bbb", AppID: "2"},
	}
	result := p.Update(context.Background(), items)
	total := len(result.Updated) + len(result.Failed)
	if total != 2 {
		t.Errorf("expected 2 items accounted for, got updated=%v failed=%v", result.Updated, result.Failed)
	}
}
