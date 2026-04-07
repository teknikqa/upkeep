// Package engine orchestrates the scan→confirm→execute→report pipeline.
package engine

import (
	"context"
	"sync"

	upkeeperrors "github.com/teknikqa/upkeep/internal/errors"
	"github.com/teknikqa/upkeep/internal/provider"
)

// ScanOptions controls the parallel scan coordinator.
type ScanOptions struct {
	// Parallelism is the max number of concurrent scans.
	Parallelism int
	// SkipLists maps provider name → set of package names to exclude from results.
	SkipLists map[string]map[string]bool
	// OnComplete is called after each provider's scan finishes with the
	// provider name and its (skip-list-filtered) ScanResult. Called from the
	// scanning goroutine — must be safe for concurrent use.
	OnComplete func(name string, result provider.ScanResult)
}

// Scan runs all provided providers in parallel (bounded by opts.Parallelism),
// collects their ScanResults, applies skip-list filtering, and returns a map
// from provider name → ScanResult.
// Respects context cancellation.
func Scan(ctx context.Context, providers []provider.Provider, opts ScanOptions) map[string]provider.ScanResult {
	if opts.Parallelism < 1 {
		opts.Parallelism = 1
	}

	results := make(map[string]provider.ScanResult, len(providers))
	var mu sync.Mutex

	sem := make(chan struct{}, opts.Parallelism)
	var wg sync.WaitGroup

	for _, p := range providers {
		wg.Add(1)
		p := p // capture loop variable
		go func() {
			defer wg.Done()

			// Check for context cancellation before acquiring the semaphore.
			select {
			case <-ctx.Done():
				mu.Lock()
				results[p.Name()] = provider.ScanResult{
					Available: false,
					Error:     ctx.Err(),
					Message:   "cancelled",
				}
				mu.Unlock()
				return
			case sem <- struct{}{}:
				defer func() { <-sem }()
			}

			result := p.Scan(ctx)

			// Wrap any scan error in a ProviderError for structured handling.
			if result.Error != nil {
				result.Error = &upkeeperrors.ProviderError{
					Provider: p.Name(),
					Phase:    "scan",
					Err:      result.Error,
				}
			}

			// Apply skip-list filtering.
			if skipSet := opts.SkipLists[p.Name()]; len(skipSet) > 0 {
				filtered := result.Outdated[:0]
				for _, item := range result.Outdated {
					if !skipSet[item.Name] {
						filtered = append(filtered, item)
					}
				}
				result.Outdated = filtered
			}

			mu.Lock()
			results[p.Name()] = result
			mu.Unlock()

			if opts.OnComplete != nil {
				opts.OnComplete(p.Name(), result)
			}
		}()
	}

	wg.Wait()
	return results
}
