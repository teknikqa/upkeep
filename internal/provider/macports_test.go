package provider_test

import (
	"context"
	"errors"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

const sampleMacportsOutdated = `git 2.39.0_0 < 2.40.0_0
zlib 1.2.11_0 < 1.2.13_0
`

func TestMacportsProvider_Name(t *testing.T) {
	p := provider.NewMacportsProvider(config.MacportsConfig{Enabled: true}, nil)
	if p.Name() != "macports" {
		t.Errorf("expected %q, got %q", "macports", p.Name())
	}
	if p.DisplayName() != "MacPorts" {
		t.Errorf("expected %q, got %q", "MacPorts", p.DisplayName())
	}
}

func TestMacportsProvider_DependsOn(t *testing.T) {
	p := provider.NewMacportsProvider(config.MacportsConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestParseMacportsOutdated(t *testing.T) {
	items := provider.ParseMacportsOutdated(sampleMacportsOutdated)
	if len(items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(items))
	}

	byName := make(map[string]provider.OutdatedItem, len(items))
	for _, item := range items {
		byName[item.Name] = item
	}

	git := byName["git"]
	if git.CurrentVersion != "2.39.0_0" {
		t.Errorf("git current: expected 2.39.0_0, got %q", git.CurrentVersion)
	}
	if git.LatestVersion != "2.40.0_0" {
		t.Errorf("git latest: expected 2.40.0_0, got %q", git.LatestVersion)
	}

	zlib := byName["zlib"]
	if zlib.CurrentVersion != "1.2.11_0" {
		t.Errorf("zlib current: expected 1.2.11_0, got %q", zlib.CurrentVersion)
	}
	if zlib.LatestVersion != "1.2.13_0" {
		t.Errorf("zlib latest: expected 1.2.13_0, got %q", zlib.LatestVersion)
	}
}

func TestParseMacportsOutdated_Empty(t *testing.T) {
	if items := provider.ParseMacportsOutdated(""); len(items) != 0 {
		t.Errorf("expected 0 items for empty output, got %d", len(items))
	}
}

func TestParseMacportsOutdated_SkipsMalformedLines(t *testing.T) {
	input := `git 2.39.0_0 < 2.40.0_0
not a valid outdated line
zlib 1.2.11_0 < 1.2.13_0
`
	items := provider.ParseMacportsOutdated(input)
	if len(items) != 2 {
		t.Fatalf("expected 2 valid items with the malformed line skipped, got %d", len(items))
	}
}

func TestMacportsProvider_Scan_WithOverride(t *testing.T) {
	tests := []struct {
		name          string
		stdout        string
		err           error
		wantAvailable bool
		wantErr       bool
		wantCount     int
	}{
		{
			name:          "outdated ports",
			stdout:        sampleMacportsOutdated,
			wantAvailable: true,
			wantCount:     2,
		},
		{
			name:          "nothing outdated",
			stdout:        "",
			wantAvailable: true,
			wantCount:     0,
		},
		{
			name:          "command error",
			err:           errors.New("port outdated failed"),
			wantAvailable: true,
			wantErr:       true,
			wantCount:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := provider.NewMacportsProvider(config.MacportsConfig{Enabled: true}, nil)
			p.SetListOutdated(func(_ context.Context) (string, error) {
				return tt.stdout, tt.err
			})

			result := p.Scan(context.Background())
			if result.Available != tt.wantAvailable {
				t.Errorf("expected Available=%v, got %v", tt.wantAvailable, result.Available)
			}
			if (result.Error != nil) != tt.wantErr {
				t.Errorf("expected error=%v, got %v", tt.wantErr, result.Error)
			}
			if len(result.Outdated) != tt.wantCount {
				t.Errorf("expected %d outdated items, got %d", tt.wantCount, len(result.Outdated))
			}
		})
	}
}

// TestMacportsProvider_Scan_RealPort exercises the real `port -q outdated`
// invocation (no listOutdated override) to cover runOutdated's default
// branch. We don't assert specific contents — the machine may or may not
// have outdated ports — only that Scan completes without panicking.
func TestMacportsProvider_Scan_RealPort(t *testing.T) {
	if !provider.CommandExistsExport("port") {
		t.Skip("port not available")
	}
	p := provider.NewMacportsProvider(config.MacportsConfig{Enabled: true}, nil)
	// No SetListOutdated — exercise the real port invocation.
	result := p.Scan(context.Background())
	if !result.Available {
		t.Errorf("expected Available=true, got false")
	}
}

func TestMacportsProvider_Scan_NoPort(t *testing.T) {
	if provider.CommandExistsExport("port") {
		t.Skip("port is installed; skipping unavailability test")
	}
	p := provider.NewMacportsProvider(config.MacportsConfig{Enabled: true}, nil)
	result := p.Scan(context.Background())
	if result.Available {
		t.Error("expected Available=false when port not installed")
	}
}

func TestMacportsProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("macports")
	if err != nil {
		t.Fatalf("macports not registered: %v", err)
	}
	if p.Name() != "macports" {
		t.Errorf("expected macports, got %s", p.Name())
	}
}

func TestMacportsProvider_Update_Empty(t *testing.T) {
	p := provider.NewMacportsProvider(config.MacportsConfig{Enabled: true}, nil)
	result := p.Update(context.Background(), nil)
	if len(result.Updated) != 0 || len(result.Failed) != 0 {
		t.Errorf("expected empty result for nil items, got updated=%v failed=%v", result.Updated, result.Failed)
	}
}

func TestMacportsProvider_Update_ItemsAccountedFor(t *testing.T) {
	p := provider.NewMacportsProvider(config.MacportsConfig{Enabled: true}, nil)
	// Fake port names that don't correspond to real installed ports, so the
	// batch and per-item fallback both fail — but every item must still be
	// accounted for. This also exercises the sudo -v pre-cache: with no tty
	// and no cached ticket it fails fast rather than hanging.
	items := []provider.OutdatedItem{
		{Name: "macports-nonexistent-port-aaa"},
		{Name: "macports-nonexistent-port-bbb"},
	}
	result := p.Update(context.Background(), items)
	total := len(result.Updated) + len(result.Failed)
	if total != 2 {
		t.Errorf("expected 2 items accounted for, got updated=%v failed=%v", result.Updated, result.Failed)
	}
}
