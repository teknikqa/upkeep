package provider_test

import (
	"context"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

const sampleComposerDryRunInstalls = `Changed current directory to /Users/nick/.composer
Loading composer repositories with package information
Updating dependencies
Lock file operations: 20 installs, 0 updates, 0 removals
  - Locking bamarni/symfony-console-autocomplete (v1.5.5)
  - Locking ergebnis/composer-normalize (2.50.0)
  - Locking ergebnis/json (1.6.0)
Installing dependencies from lock file (including require-dev)
Package operations: 3 installs, 0 updates, 0 removals
  - Installing ergebnis/json (1.6.0)
  - Installing ergebnis/composer-normalize (2.50.0)
  - Installing bamarni/symfony-console-autocomplete (v1.5.5)
1 package suggestions were added by new dependencies, use ` + "`composer suggest`" + ` to see details.
No installed packages - skipping audit.`

const sampleComposerDryRunUpdates = `Changed current directory to /Users/nick/.composer
Loading composer repositories with package information
Updating dependencies
Lock file operations: 0 installs, 2 updates, 0 removals
  - Upgrading phpunit/phpunit (10.0.0 => 10.2.1)
  - Upgrading squizlabs/php_codesniffer (3.7.1 => 3.7.2)
Installing dependencies from lock file (including require-dev)
Package operations: 0 installs, 2 updates, 0 removals
  - Updating phpunit/phpunit (10.0.0 => 10.2.1)
  - Updating squizlabs/php_codesniffer (3.7.1 => 3.7.2)`

const sampleComposerDryRunMixed = `Changed current directory to /Users/nick/.composer
Loading composer repositories with package information
Updating dependencies
Lock file operations: 1 installs, 1 updates, 0 removals
  - Locking ergebnis/json (1.6.0)
  - Upgrading phpunit/phpunit (10.0.0 => 10.2.1)
Installing dependencies from lock file (including require-dev)
Package operations: 1 installs, 1 updates, 0 removals
  - Installing ergebnis/json (1.6.0)
  - Updating phpunit/phpunit (10.0.0 => 10.2.1)`

const sampleComposerDryRunNothingToDo = `Changed current directory to /Users/nick/.composer
Loading composer repositories with package information
Updating dependencies
Nothing to modify in lock file
Installing dependencies from lock file (including require-dev)
Nothing to install, update or remove`

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

func TestParseComposerDryRun_Installs(t *testing.T) {
	items := provider.ParseComposerDryRun(sampleComposerDryRunInstalls)
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}

	// First item from the Package operations section.
	if items[0].Name != "ergebnis/json" {
		t.Errorf("expected ergebnis/json, got %s", items[0].Name)
	}
	if items[0].CurrentVersion != "" {
		t.Errorf("expected empty current version for install, got %s", items[0].CurrentVersion)
	}
	if items[0].LatestVersion != "1.6.0" {
		t.Errorf("expected latest 1.6.0, got %s", items[0].LatestVersion)
	}
}

func TestParseComposerDryRun_Updates(t *testing.T) {
	items := provider.ParseComposerDryRun(sampleComposerDryRunUpdates)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	if items[0].Name != "phpunit/phpunit" {
		t.Errorf("expected phpunit/phpunit, got %s", items[0].Name)
	}
	if items[0].CurrentVersion != "10.0.0" {
		t.Errorf("expected current 10.0.0, got %s", items[0].CurrentVersion)
	}
	if items[0].LatestVersion != "10.2.1" {
		t.Errorf("expected latest 10.2.1, got %s", items[0].LatestVersion)
	}

	if items[1].Name != "squizlabs/php_codesniffer" {
		t.Errorf("expected squizlabs/php_codesniffer, got %s", items[1].Name)
	}
}

func TestParseComposerDryRun_Mixed(t *testing.T) {
	items := provider.ParseComposerDryRun(sampleComposerDryRunMixed)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	// Install item.
	if items[0].Name != "ergebnis/json" {
		t.Errorf("expected ergebnis/json, got %s", items[0].Name)
	}
	if items[0].CurrentVersion != "" {
		t.Errorf("expected empty current version, got %s", items[0].CurrentVersion)
	}

	// Update item.
	if items[1].Name != "phpunit/phpunit" {
		t.Errorf("expected phpunit/phpunit, got %s", items[1].Name)
	}
	if items[1].CurrentVersion != "10.0.0" {
		t.Errorf("expected current 10.0.0, got %s", items[1].CurrentVersion)
	}
}

func TestParseComposerDryRun_NothingToDo(t *testing.T) {
	items := provider.ParseComposerDryRun(sampleComposerDryRunNothingToDo)
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
	}
}

func TestParseComposerDryRun_EmptyOutput(t *testing.T) {
	items := provider.ParseComposerDryRun("")
	if len(items) != 0 {
		t.Fatalf("expected 0 items, got %d", len(items))
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

func TestComposerProvider_Update_Empty(t *testing.T) {
	p := provider.NewComposerProvider(config.ComposerConfig{Enabled: true}, nil)
	result := p.Update(context.Background(), nil)
	if len(result.Updated) != 0 || len(result.Failed) != 0 {
		t.Errorf("expected empty result for nil items, got updated=%v failed=%v", result.Updated, result.Failed)
	}
}

func TestComposerProvider_Update_ItemsAccountedFor(t *testing.T) {
	p := provider.NewComposerProvider(config.ComposerConfig{Enabled: true}, nil)
	items := []provider.OutdatedItem{
		{Name: "phpunit/phpunit"},
		{Name: "squizlabs/php_codesniffer"},
	}
	result := p.Update(context.Background(), items)
	// Without composer installed, commands will fail — items should land in Failed.
	total := len(result.Updated) + len(result.Failed)
	if total != 2 {
		t.Errorf("expected 2 items accounted for, got updated=%v failed=%v", result.Updated, result.Failed)
	}
}
