package provider_test

import (
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

const samplePipOutdated = `[
  {"name": "requests", "version": "2.28.0", "latest_version": "2.31.0", "latest_filetype": "wheel"},
  {"name": "setuptools", "version": "67.0.0", "latest_version": "68.0.0", "latest_filetype": "wheel"}
]`

func TestPipProvider_Name(t *testing.T) {
	p := provider.NewPipProvider(config.PipConfig{Enabled: true}, nil)
	if p.Name() != "pip" {
		t.Errorf("expected %q, got %q", "pip", p.Name())
	}
	if p.DisplayName() != "pip / pipx" {
		t.Errorf("expected %q, got %q", "pip / pipx", p.DisplayName())
	}
}

func TestPipProvider_DependsOn(t *testing.T) {
	p := provider.NewPipProvider(config.PipConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestParsePipOutdated(t *testing.T) {
	items, err := provider.ParsePipOutdated(samplePipOutdated)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}
	if items[0].Name != "requests" {
		t.Errorf("expected requests, got %s", items[0].Name)
	}
	if items[0].CurrentVersion != "2.28.0" {
		t.Errorf("expected 2.28.0, got %s", items[0].CurrentVersion)
	}
	if items[0].LatestVersion != "2.31.0" {
		t.Errorf("expected 2.31.0, got %s", items[0].LatestVersion)
	}
}

func TestParsePipOutdated_Empty(t *testing.T) {
	items, err := provider.ParsePipOutdated("[]")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d", len(items))
	}
}

func TestPipProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("pip")
	if err != nil {
		t.Fatalf("pip not registered: %v", err)
	}
	if p.Name() != "pip" {
		t.Errorf("expected pip, got %s", p.Name())
	}
}
