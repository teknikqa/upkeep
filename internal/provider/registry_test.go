package provider_test

import (
	"context"
	"testing"
	"time"

	"github.com/teknikqa/upkeep/internal/provider"
)

// mockProvider is a test double for the Provider interface.
type mockProvider struct {
	name        string
	displayName string
	dependsOn   []string
}

func (m *mockProvider) Name() string        { return m.name }
func (m *mockProvider) DisplayName() string { return m.displayName }
func (m *mockProvider) DependsOn() []string { return m.dependsOn }
func (m *mockProvider) Scan(_ context.Context) provider.ScanResult {
	return provider.ScanResult{Available: true}
}
func (m *mockProvider) Update(_ context.Context, _ []provider.OutdatedItem) provider.UpdateResult {
	return provider.UpdateResult{Duration: time.Second}
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := provider.ExportNewRegistry()
	p := &mockProvider{name: "brew", displayName: "Homebrew"}
	r.Register(p)

	got, err := r.Get("brew")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Name() != "brew" {
		t.Errorf("expected name=brew, got %q", got.Name())
	}
}

func TestRegistry_GetUnknown(t *testing.T) {
	r := provider.ExportNewRegistry()
	_, err := r.Get("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}

func TestRegistry_ListReturnsSorted(t *testing.T) {
	r := provider.ExportNewRegistry()
	r.Register(&mockProvider{name: "npm"})
	r.Register(&mockProvider{name: "brew"})
	r.Register(&mockProvider{name: "pip"})

	names := r.List()
	expected := []string{"brew", "npm", "pip"}
	if len(names) != len(expected) {
		t.Fatalf("expected %d names, got %d", len(expected), len(names))
	}
	for i, n := range names {
		if n != expected[i] {
			t.Errorf("names[%d]: expected %q, got %q", i, expected[i], n)
		}
	}
}

func TestRegistry_GetByNames(t *testing.T) {
	r := provider.ExportNewRegistry()
	r.Register(&mockProvider{name: "brew"})
	r.Register(&mockProvider{name: "npm"})
	r.Register(&mockProvider{name: "pip"})

	ps, err := r.GetByNames([]string{"brew", "pip"})
	if err != nil {
		t.Fatalf("GetByNames: %v", err)
	}
	if len(ps) != 2 {
		t.Errorf("expected 2 providers, got %d", len(ps))
	}
}

func TestRegistry_GetByNamesUnknown(t *testing.T) {
	r := provider.ExportNewRegistry()
	r.Register(&mockProvider{name: "brew"})

	_, err := r.GetByNames([]string{"brew", "nonexistent"})
	if err == nil {
		t.Fatal("expected error for unknown provider name, got nil")
	}
}

func TestRegistry_DuplicateRegisterPanics(t *testing.T) {
	r := provider.ExportNewRegistry()
	r.Register(&mockProvider{name: "brew"})

	defer func() {
		if rec := recover(); rec == nil {
			t.Fatal("expected panic for duplicate registration")
		}
	}()
	r.Register(&mockProvider{name: "brew"})
}
