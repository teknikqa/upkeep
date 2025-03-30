package provider_test

import (
	"context"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

func TestVirtualBoxProvider_Name(t *testing.T) {
	p := provider.NewVirtualBoxProvider(
		config.VirtualBoxConfig{Enabled: true, Notify: false},
		config.NotificationsConfig{Enabled: false},
		nil,
	)
	if p.Name() != "virtualbox" {
		t.Errorf("expected %q, got %q", "virtualbox", p.Name())
	}
	if p.DisplayName() != "VirtualBox" {
		t.Errorf("expected %q, got %q", "VirtualBox", p.DisplayName())
	}
}

func TestVirtualBoxProvider_DependsOn(t *testing.T) {
	p := provider.NewVirtualBoxProvider(
		config.VirtualBoxConfig{Enabled: true},
		config.NotificationsConfig{Enabled: false},
		nil,
	)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestParseVirtualBoxVersion(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"7.0.10r158379\n", "7.0.10"},
		{"6.1.44\n", "6.1.44"},
		{"7.0.10", "7.0.10"},
	}
	for _, tc := range cases {
		got := provider.ParseVirtualBoxVersion(tc.input)
		if got != tc.expected {
			t.Errorf("ParseVirtualBoxVersion(%q): expected %q, got %q", tc.input, tc.expected, got)
		}
	}
}

func TestVirtualBoxProvider_Update_SkipsAll(t *testing.T) {
	p := provider.NewVirtualBoxProvider(
		config.VirtualBoxConfig{Enabled: true, Notify: false},
		config.NotificationsConfig{Enabled: false},
		nil,
	)
	items := []provider.OutdatedItem{
		{Name: "virtualbox", CurrentVersion: "7.0.10", LatestVersion: "7.0.14"},
	}
	result := p.Update(context.Background(), items)
	if len(result.Skipped) != 1 {
		t.Errorf("expected 1 skipped, got %v", result.Skipped)
	}
}

func TestVirtualBoxProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("virtualbox")
	if err != nil {
		t.Fatalf("virtualbox not registered: %v", err)
	}
	if p.Name() != "virtualbox" {
		t.Errorf("expected virtualbox, got %s", p.Name())
	}
}
