// Package engine orchestrates the scan→confirm→execute→report pipeline.
// The executor implements a dependency-graph based parallel execution model:
// providers whose DependsOn() dependencies are satisfied are started immediately,
// and each completing provider recursively wakes any newly-ready dependents.
package engine

import (
	"context"
	"sync"

	upkeeperrors "github.com/teknikqa/upkeep/internal/errors"
	"github.com/teknikqa/upkeep/internal/provider"
)

// ExecuteOptions controls parallel execution.
type ExecuteOptions struct {
	// Parallelism is the max number of concurrent provider executions.
	Parallelism int
	// OnStart is called just before each provider begins updating (after
	// dependencies are met and a semaphore slot is acquired).
	OnStart func(name string)
	// OnComplete is called after each provider finishes (for live table updates, etc.).
	OnComplete func(name string, result provider.UpdateResult)
	// OnProgress is called after each individual package completes within a provider.
	// The provider name is prepended so the UI knows which provider the progress belongs to.
	OnProgress func(providerName string, progress provider.PackageProgress)
}

// Execute runs all provided providers, respecting dependency ordering from
// DependsOn(), bounded by opts.Parallelism.
// Provider failures do not cancel other providers.
// Returns a map from provider name → UpdateResult.
func Execute(ctx context.Context, providers []provider.Provider, scanResults map[string]provider.ScanResult, opts ExecuteOptions) map[string]provider.UpdateResult {
	if opts.Parallelism < 1 {
		opts.Parallelism = 1
	}

	results := make(map[string]provider.UpdateResult, len(providers))
	done := make(map[string]bool, len(providers))
	var mu sync.Mutex

	// Build a map by name for fast lookup.
	byName := make(map[string]provider.Provider, len(providers))
	for _, p := range providers {
		byName[p.Name()] = p
	}

	sem := make(chan struct{}, opts.Parallelism)
	var wg sync.WaitGroup

	// pending tracks providers not yet started.
	// We use a simple loop that repeatedly checks which providers are ready.
	pending := make(map[string]bool, len(providers))
	for _, p := range providers {
		pending[p.Name()] = true
	}

	// startProvider launches a provider in a goroutine when all its dependencies
	// have completed. It respects context cancellation both while waiting for a
	// semaphore slot and immediately before calling Update, so in-flight goroutines
	// drain quickly when the user cancels.
	var startProvider func(p provider.Provider)
	startProvider = func(p provider.Provider) {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Wait for a semaphore slot, but bail out if context is cancelled.
			select {
			case <-ctx.Done():
				mu.Lock()
				results[p.Name()] = provider.UpdateResult{Error: ctx.Err()}
				mu.Unlock()
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()

			// Check context again after acquiring semaphore (it may have been
			// cancelled while we were waiting).
			if ctx.Err() != nil {
				mu.Lock()
				results[p.Name()] = provider.UpdateResult{Error: ctx.Err()}
				mu.Unlock()
				return
			}

			// Notify that this provider is starting.
			if opts.OnStart != nil {
				opts.OnStart(p.Name())
			}

			// Inject per-package progress callback into context so providers
			// can report incremental progress without changing the Provider interface.
			updateCtx := ctx
			if opts.OnProgress != nil {
				pName := p.Name()
				updateCtx = provider.ContextWithProgress(ctx, func(prog provider.PackageProgress) {
					opts.OnProgress(pName, prog)
				})
			}

			// Get the items to update from the scan results.
			scanResult := scanResults[p.Name()]
			result := p.Update(updateCtx, scanResult.Outdated)

			// Wrap any update error in a ProviderError for structured handling.
			if result.Error != nil {
				result.Error = &upkeeperrors.ProviderError{
					Provider: p.Name(),
					Phase:    "update",
					Err:      result.Error,
				}
			}

			mu.Lock()
			results[p.Name()] = result
			done[p.Name()] = true
			mu.Unlock()

			if opts.OnComplete != nil {
				opts.OnComplete(p.Name(), result)
			}

			// After this provider finishes, check if any pending provider is now ready.
			mu.Lock()
			toStart := []provider.Provider{}
			for name := range pending {
				p2 := byName[name]
				if depsComplete(p2.DependsOn(), done) {
					delete(pending, name)
					toStart = append(toStart, p2)
				}
			}
			mu.Unlock()

			for _, p2 := range toStart {
				startProvider(p2)
			}
		}()
	}

	// Kick off providers whose dependencies are already met (initially those with none).
	mu.Lock()
	toStart := []provider.Provider{}
	for name := range pending {
		p := byName[name]
		if depsComplete(p.DependsOn(), done) {
			delete(pending, name)
			toStart = append(toStart, p)
		}
	}
	mu.Unlock()

	for _, p := range toStart {
		startProvider(p)
	}

	wg.Wait()

	// Any providers still in pending (broken dependency graph) get a failed result.
	mu.Lock()
	for name := range pending {
		results[name] = provider.UpdateResult{
			Error: provider.ErrDependencyNotMet,
		}
	}
	mu.Unlock()

	return results
}

// depsComplete returns true if all named dependencies are in the done set.
func depsComplete(deps []string, done map[string]bool) bool {
	for _, dep := range deps {
		if !done[dep] {
			return false
		}
	}
	return true
}
