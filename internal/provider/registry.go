package provider

import (
	"fmt"
	"sort"
	"sync"
)

// registry is the package-level singleton registry.
var globalRegistry = &Registry{
	providers: make(map[string]Provider),
}

// Registry holds all registered providers.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]Provider
}

// Register adds a provider to the global registry.
// Providers call this from their init() function to self-register at startup.
// Panics if a provider with the same name is already registered.
func Register(p Provider) {
	globalRegistry.Register(p)
}

// Get returns the provider with the given name from the global registry.
// Returns an error if no provider with that name is registered.
func Get(name string) (Provider, error) {
	return globalRegistry.Get(name)
}

// List returns a sorted list of all registered provider names.
// Provider names are the canonical machine-readable identifiers (e.g., "brew", "npm").
func List() []string {
	return globalRegistry.List()
}

// GetAll returns all registered providers (order not guaranteed).
// Providers self-register via init(), so the set is populated at package import time.
func GetAll() []Provider {
	return globalRegistry.GetAll()
}

// GetByNames returns providers matching the given names.
// Returns an error if any name is not found.
func GetByNames(names []string) ([]Provider, error) {
	return globalRegistry.GetByNames(names)
}

// Register adds a provider to the registry.
// Panics if a provider with the same name is already registered.
func (r *Registry) Register(p Provider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := p.Name()
	if _, exists := r.providers[name]; exists {
		panic(fmt.Sprintf("provider %q already registered", name))
	}
	r.providers[name] = p
}

// Get returns the provider with the given name.
func (r *Registry) Get(name string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider %q not found", name)
	}
	return p, nil
}

// List returns a sorted list of all registered provider names.
func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// GetAll returns all registered providers.
func (r *Registry) GetAll() []Provider {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ps := make([]Provider, 0, len(r.providers))
	for _, p := range r.providers {
		ps = append(ps, p)
	}
	return ps
}

// GetByNames returns providers matching the given names.
func (r *Registry) GetByNames(names []string) ([]Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ps := make([]Provider, 0, len(names))
	for _, name := range names {
		p, ok := r.providers[name]
		if !ok {
			return nil, fmt.Errorf("provider %q not found", name)
		}
		ps = append(ps, p)
	}
	return ps, nil
}

// newRegistry creates a fresh (empty) Registry, for testing.
func newRegistry() *Registry {
	return &Registry{providers: make(map[string]Provider)}
}
