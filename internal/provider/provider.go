// Package provider defines the Provider interface and shared types.
package provider

import (
	"context"
	"errors"
	"time"
)

// PackageStatus represents the outcome of a single package update.
type PackageStatus string

const (
	// PackageUpdated means the package was successfully updated.
	PackageUpdated PackageStatus = "updated"
	// PackageFailed means the package update failed.
	PackageFailed PackageStatus = "failed"
	// PackageDeferred means the package was deferred (e.g., auth-required).
	PackageDeferred PackageStatus = "deferred"
	// PackageSkipped means the package was skipped (e.g., config skip-list).
	PackageSkipped PackageStatus = "skipped"
	// PackageStarting means the package is about to be processed.
	PackageStarting PackageStatus = "starting"
)

// PackageProgress reports the completion of a single package within a provider.
type PackageProgress struct {
	Name   string        // package name
	Status PackageStatus // outcome
}

// ProgressFunc is called by providers after each individual package completes.
// Providers retrieve it from context via ProgressFromContext.
type ProgressFunc func(progress PackageProgress)

// progressKey is the context key for ProgressFunc.
type progressKey struct{}

// ContextWithProgress returns a new context carrying the given ProgressFunc.
func ContextWithProgress(ctx context.Context, fn ProgressFunc) context.Context {
	return context.WithValue(ctx, progressKey{}, fn)
}

// ProgressFromContext retrieves the ProgressFunc from ctx, or nil if not set.
func ProgressFromContext(ctx context.Context) ProgressFunc {
	fn, _ := ctx.Value(progressKey{}).(ProgressFunc)
	return fn
}

// ReportProgress is a convenience helper that calls the ProgressFunc from ctx
// if one is set. Safe to call when ctx has no ProgressFunc.
func ReportProgress(ctx context.Context, name string, status PackageStatus) {
	if fn := ProgressFromContext(ctx); fn != nil {
		fn(PackageProgress{Name: name, Status: status})
	}
}

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

// SubGrouper is an optional interface that providers can implement to declare
// their sub-group labels upfront (before scanning). This allows the UI to
// show sub-group rows in the scan table immediately rather than waiting for
// scan results.
type SubGrouper interface {
	// SubGroups returns the labels of sub-groups this provider will report.
	SubGroups() []string
}
