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

// --- Registry.GetAll ---

func TestRegistry_GetAll(t *testing.T) {
	r := provider.ExportNewRegistry()
	r.Register(&mockProvider{name: "brew"})
	r.Register(&mockProvider{name: "npm"})
	r.Register(&mockProvider{name: "pip"})

	all := r.GetAll()
	if len(all) != 3 {
		t.Errorf("expected 3 providers, got %d", len(all))
	}
}

// --- Global registry functions ---

func TestGlobal_List_ReturnsRegisteredProviders(t *testing.T) {
	names := provider.List()
	if len(names) == 0 {
		t.Fatal("expected at least one registered provider")
	}
	// List should be sorted.
	for i := 1; i < len(names); i++ {
		if names[i] < names[i-1] {
			t.Errorf("List() not sorted: %q < %q at index %d", names[i], names[i-1], i)
		}
	}
}

func TestGlobal_Get_KnownProvider(t *testing.T) {
	// "brew" is registered via init().
	p, err := provider.Get("brew")
	if err != nil {
		t.Fatalf("Get(\"brew\"): %v", err)
	}
	if p.Name() != "brew" {
		t.Errorf("expected provider name 'brew', got %q", p.Name())
	}
}

func TestGlobal_Get_UnknownProvider(t *testing.T) {
	_, err := provider.Get("nonexistent-provider-xyz")
	if err == nil {
		t.Fatal("expected error for unknown provider, got nil")
	}
}

func TestGlobal_GetAll_LengthMatchesList(t *testing.T) {
	all := provider.GetAll()
	names := provider.List()
	if len(all) != len(names) {
		t.Errorf("GetAll() len=%d, List() len=%d — expected equal", len(all), len(names))
	}
}

func TestGlobal_GetByNames_Subset(t *testing.T) {
	names := provider.List()
	if len(names) < 2 {
		t.Skip("need at least 2 providers for subset test")
	}
	subset := names[:2]
	ps, err := provider.GetByNames(subset)
	if err != nil {
		t.Fatalf("GetByNames: %v", err)
	}
	if len(ps) != 2 {
		t.Errorf("expected 2 providers, got %d", len(ps))
	}
}

// TestProvider_DisplayNameAndDependsOn verifies that every registered provider
// returns the expected human-readable display name and dependency list.
func TestProvider_DisplayNameAndDependsOn(t *testing.T) {
	tests := []struct {
		name        string
		displayName string
		deps        []string
	}{
		{"brew", "Homebrew Formulae", nil},
		{"brew-cask", "Homebrew Casks", []string{"brew"}},
		{"composer", "Composer Global Packages", nil},
		{"editor", "Code Editor Extensions", nil},
		{"npm", "npm Global Packages", nil},
		{"omz", "Oh My Zsh", nil},
		{"pip", "pip / pipx", nil},
		{"rust", "Rust (rustup + cargo)", nil},
		{"vagrant", "Vagrant", nil},
		{"vim", "Vim Plugins", nil},
		{"virtualbox", "VirtualBox", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := provider.Get(tt.name)
			if err != nil {
				t.Fatalf("Get(%q): %v", tt.name, err)
			}
			if got := p.DisplayName(); got != tt.displayName {
				t.Errorf("DisplayName() = %q, want %q", got, tt.displayName)
			}
			deps := p.DependsOn()
			if len(deps) != len(tt.deps) {
				t.Errorf("DependsOn() len = %d, want %d; got %v", len(deps), len(tt.deps), deps)
				return
			}
			for i, d := range deps {
				if d != tt.deps[i] {
					t.Errorf("DependsOn()[%d] = %q, want %q", i, d, tt.deps[i])
				}
			}
		})
	}
}
