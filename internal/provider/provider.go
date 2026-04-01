// Package provider defines the Provider interface and shared types.
package provider

import (
	"context"
	"errors"
	"time"
)

// ErrDependencyNotMet is returned when a provider's dependency was not satisfied.
var ErrDependencyNotMet = errors.New("provider dependency not met")

// OutdatedItem represents a single package that has an update available.
type OutdatedItem struct {
	Name           string
	CurrentVersion string
	LatestVersion  string
	AuthRequired   bool // only relevant for brew-cask
}

// ScanResult is returned by Provider.Scan().
type ScanResult struct {
	Available    bool                // is the tool installed on this system?
	Outdated     []OutdatedItem      // packages with updates available
	AlwaysUpdate bool                // run Update even when Outdated is empty (e.g. editor)
	Error        error               // non-fatal scan error
	Message      string              // human-readable status message
	Groups       map[string][]string // optional: group label → package names for sub-grouped display
}

// UpdateResult is returned by Provider.Update().
type UpdateResult struct {
	Updated  []string      // packages successfully updated
	Deferred []string      // packages deferred (auth-required)
	Skipped  []string      // packages skipped (config skip-list)
	Failed   []string      // packages that failed to update
	Error    error         // overall error (nil if partial success)
	Duration time.Duration // wall-clock time for this provider
	LogFile  string        // path to captured log output
}

// Provider is the interface every updater must implement.
type Provider interface {
	// Name returns the provider's identifier (e.g., "brew", "npm").
	Name() string

	// DisplayName returns a human-friendly name (e.g., "Homebrew Formulae").
	DisplayName() string

	// DependsOn returns names of providers that must complete before this one.
	DependsOn() []string

	// Scan checks what's outdated. Called during the scan phase.
	Scan(ctx context.Context) ScanResult

	// Update performs the actual updates. Called during the execute phase.
	// The items slice contains only the items the user confirmed for update.
	Update(ctx context.Context, items []OutdatedItem) UpdateResult
}
