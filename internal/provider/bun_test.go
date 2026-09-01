package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

// sampleBunOutdatedTable is real `bun outdated -g` stdout, captured against
// bun 1.4.0.
const sampleBunOutdatedTable = `bun outdated v1.4.0 (1381054db)
|---------------------------------------|
| Package  | Current | Update  | Latest |
|----------|---------|---------|--------|
| cowsay   | 1.5.0   | 1.5.0   | 1.6.0  |
|----------|---------|---------|--------|
| lodash   | 4.17.20 | 4.17.20 | 4.18.1 |
|---------------------------------------|
`

// sampleBunEmptyOutput is real stdout when `bun outdated -g` finds nothing
// outdated (global packages exist but none are stale).
const sampleBunEmptyOutput = "bun outdated v1.4.0 (1381054db)\n"

// sampleBunNoLockfileStderr is the real stderr from `bun outdated -g` when
// no global packages are installed yet (no global lockfile).
const sampleBunNoLockfileStderr = `error: missing lockfile, nothing outdated
note: run 'bun install' first`

func TestBunProvider_Name(t *testing.T) {
	p := provider.NewBunProvider(config.BunConfig{Enabled: true}, nil)
	if p.Name() != "bun" {
		t.Errorf("expected %q, got %q", "bun", p.Name())
	}
	if p.DisplayName() != "Bun Global Packages" {
		t.Errorf("expected %q, got %q", "Bun Global Packages", p.DisplayName())
	}
}

func TestBunProvider_DependsOn(t *testing.T) {
	p := provider.NewBunProvider(config.BunConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestBunProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("bun")
	if err != nil {
		t.Fatalf("bun not registered: %v", err)
	}
	if p.Name() != "bun" {
		t.Errorf("expected bun, got %s", p.Name())
	}
}

func TestParseBunOutdatedTable(t *testing.T) {
	items := provider.ParseBunOutdatedTable(sampleBunOutdatedTable)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d: %+v", len(items), items)
	}

	names := map[string]provider.OutdatedItem{}
	for _, item := range items {
		names[item.Name] = item
	}

	cowsay, ok := names["cowsay"]
	if !ok {
		t.Fatal("expected cowsay in outdated list")
	}
	if cowsay.CurrentVersion != "1.5.0" || cowsay.LatestVersion != "1.6.0" {
		t.Errorf("cowsay: expected 1.5.0 -> 1.6.0, got %s -> %s", cowsay.CurrentVersion, cowsay.LatestVersion)
	}

	lodash, ok := names["lodash"]
	if !ok {
		t.Fatal("expected lodash in outdated list")
	}
	if lodash.CurrentVersion != "4.17.20" || lodash.LatestVersion != "4.18.1" {
		t.Errorf("lodash: expected 4.17.20 -> 4.18.1, got %s -> %s", lodash.CurrentVersion, lodash.LatestVersion)
	}
}

func TestParseBunOutdatedTable_Empty(t *testing.T) {
	items := provider.ParseBunOutdatedTable(sampleBunEmptyOutput)
	if len(items) != 0 {
		t.Errorf("expected 0 items, got %d: %+v", len(items), items)
	}
}

func TestIsBunNoGlobalLockfile(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{"real error", sampleBunNoLockfileStderr, true},
		{"unrelated error", "network request failed", false},
		{"empty", "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := provider.IsBunNoGlobalLockfile(tc.output); got != tc.want {
				t.Errorf("IsBunNoGlobalLockfile(%q) = %v, want %v", tc.output, got, tc.want)
			}
		})
	}
}

func TestBunProvider_Scan_NotAvailable(t *testing.T) {
	p := provider.NewBunProvider(config.BunConfig{Enabled: true}, nil)
	p.SetCheckAvailable(func() bool { return false })

	result := p.Scan(context.Background())
	if result.Available {
		t.Error("expected Available=false")
	}
}

func TestBunProvider_Scan_NoGlobalLockfile(t *testing.T) {
	p := provider.NewBunProvider(config.BunConfig{Enabled: true}, nil)
	p.SetCheckAvailable(func() bool { return true })
	p.SetRunOutdated(func(_ context.Context) (string, string, error) {
		return "bun outdated v1.4.0 (1381054db)\n", sampleBunNoLockfileStderr, errors.New("exit status 1")
	})

	result := p.Scan(context.Background())
	if !result.Available {
		t.Fatal("expected Available=true")
	}
	if len(result.Outdated) != 0 {
		t.Errorf("expected 0 outdated, got %d", len(result.Outdated))
	}
}

func TestBunProvider_Scan_Outdated(t *testing.T) {
	tests := []struct {
		name      string
		stdout    string
		wantCount int
	}{
		{"populated", sampleBunOutdatedTable, 2},
		{"empty", sampleBunEmptyOutput, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := provider.NewBunProvider(config.BunConfig{Enabled: true}, nil)
			p.SetCheckAvailable(func() bool { return true })
			p.SetRunOutdated(func(_ context.Context) (string, string, error) {
				return tt.stdout, "", nil
			})

			result := p.Scan(context.Background())
			if !result.Available {
				t.Fatal("expected Available=true")
			}
			if len(result.Outdated) != tt.wantCount {
				t.Errorf("expected %d outdated, got %d", tt.wantCount, len(result.Outdated))
			}
		})
	}
}

func TestBunProvider_Update_Empty(t *testing.T) {
	p := provider.NewBunProvider(config.BunConfig{Enabled: true}, nil)
	result := p.Update(context.Background(), nil)
	if len(result.Updated) != 0 || len(result.Failed) != 0 {
		t.Errorf("expected empty result, got %+v", result)
	}
}

func TestBunProvider_Update_Batch(t *testing.T) {
	p := provider.NewBunProvider(config.BunConfig{Enabled: true}, nil)
	var gotNames []string
	p.SetRunUpdate(func(_ context.Context, names []string) (string, error) {
		gotNames = append(gotNames, names...)
		return "", nil
	})

	items := []provider.OutdatedItem{
		{Name: "cowsay", CurrentVersion: "1.5.0", LatestVersion: "1.6.0"},
		{Name: "lodash", CurrentVersion: "4.17.20", LatestVersion: "4.18.1"},
	}
	result := p.Update(context.Background(), items)

	if len(result.Updated) != 2 {
		t.Errorf("expected 2 updated, got %v", result.Updated)
	}
	if len(result.Failed) != 0 {
		t.Errorf("expected 0 failed, got %v", result.Failed)
	}
	if len(gotNames) != 2 {
		t.Errorf("expected doUpdate to see 2 names, got %v", gotNames)
	}
}

func TestBunProvider_Update_BatchFailsFallsBackPerPackage(t *testing.T) {
	p := provider.NewBunProvider(config.BunConfig{Enabled: true}, nil)
	p.SetRunUpdate(func(_ context.Context, names []string) (string, error) {
		if len(names) > 1 {
			return "", errors.New("batch failed")
		}
		if names[0] == "bad-pkg" {
			return "", errors.New("not found")
		}
		return "", nil
	})

	items := []provider.OutdatedItem{
		{Name: "cowsay"},
		{Name: "bad-pkg"},
	}
	result := p.Update(context.Background(), items)

	if len(result.Updated) != 1 || result.Updated[0] != "cowsay" {
		t.Errorf("expected [cowsay] updated, got %v", result.Updated)
	}
	if len(result.Failed) != 1 || result.Failed[0] != "bad-pkg" {
		t.Errorf("expected [bad-pkg] failed, got %v", result.Failed)
	}
}
