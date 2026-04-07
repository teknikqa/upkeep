package ui_test

import (
	"sync"
	"testing"

	"github.com/teknikqa/upkeep/internal/provider"
	"github.com/teknikqa/upkeep/internal/ui"
)

// TestLiveScanTable_NonTTY_BuildsSummaryRows verifies that Stop() returns
// correct ScanSummaryRows after all providers have completed their scans.
func TestLiveScanTable_NonTTY_BuildsSummaryRows(t *testing.T) {
	order := []string{"brew", "npm", "pip"}
	displayNames := map[string]string{
		"brew": "Homebrew Formulae",
		"npm":  "npm Global Packages",
		"pip":  "pip / pipx",
	}

	lst := ui.NewLiveScanTable(order, displayNames, nil)

	// Simulate scan completions.
	lst.OnScanComplete("brew", provider.ScanResult{
		Available: true,
		Outdated:  []provider.OutdatedItem{{Name: "git"}, {Name: "jq"}},
	})
	lst.OnScanComplete("npm", provider.ScanResult{
		Available: true,
		Outdated:  []provider.OutdatedItem{},
	})
	lst.OnScanComplete("pip", provider.ScanResult{
		Available: false,
		Message:   "pip not installed",
	})

	rows, _ := lst.Stop()

	if len(rows) != 3 {
		t.Fatalf("expected 3 rows, got %d", len(rows))
	}

	// Verify order is preserved.
	if rows[0].ProviderName != "brew" || rows[1].ProviderName != "npm" || rows[2].ProviderName != "pip" {
		t.Errorf("unexpected row order: %v, %v, %v", rows[0].ProviderName, rows[1].ProviderName, rows[2].ProviderName)
	}

	// Verify brew row.
	if rows[0].DisplayName != "Homebrew Formulae" {
		t.Errorf("brew: expected display name 'Homebrew Formulae', got %q", rows[0].DisplayName)
	}
	if rows[0].OutdatedCount != 2 {
		t.Errorf("brew: expected 2 outdated, got %d", rows[0].OutdatedCount)
	}
	if !rows[0].Available {
		t.Error("brew: expected Available=true")
	}

	// Verify pip row.
	if rows[2].Available {
		t.Error("pip: expected Available=false")
	}
	if rows[2].OutdatedCount != 0 {
		t.Errorf("pip: expected 0 outdated, got %d", rows[2].OutdatedCount)
	}
}

// TestLiveScanTable_ConcurrentAccess verifies no data races when multiple
// goroutines call OnScanComplete concurrently. Run with `go test -race`.
func TestLiveScanTable_ConcurrentAccess(t *testing.T) {
	names := []string{"a", "b", "c", "d", "e"}
	displayNames := make(map[string]string, len(names))
	for _, n := range names {
		displayNames[n] = n
	}

	lst := ui.NewLiveScanTable(names, displayNames, nil)

	var wg sync.WaitGroup
	for _, name := range names {
		name := name
		wg.Add(1)
		go func() {
			defer wg.Done()
			lst.OnScanComplete(name, provider.ScanResult{
				Available: true,
				Outdated:  []provider.OutdatedItem{{Name: "pkg-" + name}},
			})
		}()
	}
	wg.Wait()

	rows, _ := lst.Stop()

	if len(rows) != 5 {
		t.Fatalf("expected 5 rows, got %d", len(rows))
	}
}

// TestLiveScanTable_StopIdempotent verifies Stop can be called multiple times.
func TestLiveScanTable_StopIdempotent(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Stop panicked: %v", r)
		}
	}()

	lst := ui.NewLiveScanTable([]string{"brew"}, map[string]string{"brew": "Homebrew"}, nil)
	lst.OnScanComplete("brew", provider.ScanResult{Available: true})
	lst.Stop()
	lst.Stop()
	lst.Stop()
}

// TestLiveScanTable_UnknownProvider verifies that callbacks for unknown
// provider names don't panic.
func TestLiveScanTable_UnknownProvider(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("unexpected panic: %v", r)
		}
	}()

	lst := ui.NewLiveScanTable([]string{"brew"}, map[string]string{"brew": "Homebrew"}, nil)
	lst.OnScanComplete("nonexistent", provider.ScanResult{Available: true})
	lst.Stop()
}

// TestLiveScanTable_IncompleteProviders verifies that providers which never
// complete are handled gracefully (marked as unavailable).
func TestLiveScanTable_IncompleteProviders(t *testing.T) {
	lst := ui.NewLiveScanTable(
		[]string{"brew", "npm"},
		map[string]string{"brew": "Homebrew", "npm": "npm"},
		nil,
	)

	// Only complete brew, leave npm scanning.
	lst.OnScanComplete("brew", provider.ScanResult{Available: true})

	rows, _ := lst.Stop()

	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}

	// npm should be marked as unavailable since it never completed.
	if rows[1].Available {
		t.Error("npm: expected Available=false for incomplete provider")
	}
}

// TestLiveScanTable_WithGroups verifies that grouped scan results are
// preserved through the LiveScanTable.
func TestLiveScanTable_WithGroups(t *testing.T) {
	lst := ui.NewLiveScanTable(
		[]string{"editor"},
		map[string]string{"editor": "Code Editor Extensions"},
		nil,
	)

	lst.OnScanComplete("editor", provider.ScanResult{
		Available: true,
		Outdated:  []provider.OutdatedItem{{Name: "ms-python.python"}},
		Groups: map[string][]string{
			"cursor": {"ms-python.python"},
		},
	})

	rows, _ := lst.Stop()

	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if len(rows[0].PackageGroups) == 0 {
		t.Error("expected PackageGroups to be populated")
	}
	if pkgs, ok := rows[0].PackageGroups["cursor"]; !ok || len(pkgs) != 1 {
		t.Errorf("expected cursor group with 1 package, got %v", rows[0].PackageGroups)
	}
}
