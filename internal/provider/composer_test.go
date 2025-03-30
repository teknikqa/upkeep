package provider_test

import (
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

const sampleComposerOutdated = `{
  "installed": [
    {
      "name": "phpunit/phpunit",
      "version": "10.0.0",
      "latest": "10.2.1",
      "latest-status": "semver-safe-update",
      "description": "The PHP Unit Testing framework"
    },
    {
      "name": "squizlabs/php_codesniffer",
      "version": "3.7.1",
      "latest": "3.7.2",
      "latest-status": "semver-safe-update",
      "description": "PHP_CodeSniffer"
    }
  ]
}`

func TestComposerProvider_Name(t *testing.T) {
	p := provider.NewComposerProvider(config.ComposerConfig{Enabled: true}, nil)
	if p.Name() != "composer" {
		t.Errorf("expected %q, got %q", "composer", p.Name())
	}
	if p.DisplayName() != "Composer Global Packages" {
		t.Errorf("expected %q, got %q", "Composer Global Packages", p.DisplayName())
	}
}

func TestComposerProvider_DependsOn(t *testing.T) {
	p := provider.NewComposerProvider(config.ComposerConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestParseComposerOutdated(t *testing.T) {
	items, err := provider.ParseComposerOutdated(sampleComposerOutdated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "phpunit/phpunit" {
		t.Errorf("expected phpunit/phpunit, got %s", items[0].Name)
	}
	if items[0].CurrentVersion != "10.0.0" {
		t.Errorf("expected version 10.0.0, got %s", items[0].CurrentVersion)
	}
	if items[0].LatestVersion != "10.2.1" {
		t.Errorf("expected latest 10.2.1, got %s", items[0].LatestVersion)
	}
}

func TestComposerProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("composer")
	if err != nil {
		t.Fatalf("composer not registered: %v", err)
	}
	if p.Name() != "composer" {
		t.Errorf("expected composer, got %s", p.Name())
	}
}
