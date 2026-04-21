package provider_test

import (
	"context"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

const brewOutdatedJSON = `{
  "formulae": [
    {
      "name": "git",
      "installed_versions": ["2.39.0"],
      "current_version": "2.40.0"
    },
    {
      "name": "jq",
      "installed_versions": ["1.6"],
      "current_version": "1.7"
    }
  ],
  "casks": []
}`

func TestBrewProvider_ParseOutdated(t *testing.T) {
	p := provider.NewBrewProvider(config.BrewConfig{Enabled: true}, nil)
	items := provider.ExportParseBrewOutdated(p, brewOutdatedJSON)

	if len(items) != 2 {
		t.Fatalf("expected 2 outdated formulae, got %d", len(items))
	}

	if items[0].Name != "git" {
		t.Errorf("expected first item name=git, got %q", items[0].Name)
	}
	if items[0].CurrentVersion != "2.39.0" {
		t.Errorf("expected git current=2.39.0, got %q", items[0].CurrentVersion)
	}
	if items[0].LatestVersion != "2.40.0" {
		t.Errorf("expected git latest=2.40.0, got %q", items[0].LatestVersion)
	}
}

func TestBrewProvider_ParseOutdated_Empty(t *testing.T) {
	p := provider.NewBrewProvider(config.BrewConfig{Enabled: true}, nil)
	items := provider.ExportParseBrewOutdated(p, `{"formulae":[],"casks":[]}`)
	if len(items) != 0 {
		t.Errorf("expected 0 items for empty JSON, got %d", len(items))
	}
}

func TestBrewProvider_ParseOutdated_InvalidJSON(t *testing.T) {
	p := provider.NewBrewProvider(config.BrewConfig{Enabled: true}, nil)
	items := provider.ExportParseBrewOutdated(p, `not valid json`)
	if items != nil {
		t.Error("expected nil items for invalid JSON")
	}
}

func TestBrewProvider_Update_Empty(t *testing.T) {
	p := provider.NewBrewProvider(config.BrewConfig{Enabled: true}, nil)
	result := p.Update(context.Background(), nil)
	if len(result.Updated) != 0 || len(result.Failed) != 0 {
		t.Errorf("expected empty result for nil items, got updated=%v failed=%v", result.Updated, result.Failed)
	}
}

func TestBrewProvider_Update_ItemsAccountedFor(t *testing.T) {
	p := provider.NewBrewProvider(config.BrewConfig{Enabled: true}, nil)
	items := []provider.OutdatedItem{
		{Name: "git"},
		{Name: "jq"},
	}
	result := p.Update(context.Background(), items)
	// Without brew installed, commands will fail — items should land in Failed.
	// Either way, every item must be accounted for.
	total := len(result.Updated) + len(result.Failed)
	if total != 2 {
		t.Errorf("expected 2 items accounted for, got updated=%v failed=%v", result.Updated, result.Failed)
	}
}

func TestBrewProvider_Update_WithPostHooks(t *testing.T) {
	p := provider.NewBrewProvider(config.BrewConfig{
		Enabled:   true,
		PostHooks: []string{"echo post-hook-ran"},
	}, nil)
	items := []provider.OutdatedItem{{Name: "git"}}
	// Should not panic even if brew isn't available.
	result := p.Update(context.Background(), items)
	total := len(result.Updated) + len(result.Failed)
	if total != 1 {
		t.Errorf("expected 1 item accounted for, got updated=%v failed=%v", result.Updated, result.Failed)
	}
}
