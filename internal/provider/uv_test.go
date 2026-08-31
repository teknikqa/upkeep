package provider_test

import (
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

func TestUvProvider_Name(t *testing.T) {
	p := provider.NewUvProvider(config.UvConfig{Enabled: true}, nil)
	if p.Name() != "uv" {
		t.Errorf("expected %q, got %q", "uv", p.Name())
	}
	if p.DisplayName() != "uv (Python)" {
		t.Errorf("expected %q, got %q", "uv (Python)", p.DisplayName())
	}
}

func TestUvProvider_DependsOn(t *testing.T) {
	p := provider.NewUvProvider(config.UvConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestUvProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("uv")
	if err != nil {
		t.Fatalf("uv not registered: %v", err)
	}
	if p.Name() != "uv" {
		t.Errorf("expected uv, got %s", p.Name())
	}
}

func TestIsUvManagedInstall(t *testing.T) {
	cases := []struct {
		name   string
		output string
		want   bool
	}{
		{
			name:   "managed install message",
			output: "error: uv was installed through an external package manager and cannot update itself.",
			want:   true,
		},
		{
			name:   "unrelated error",
			output: "error: network connection failed",
			want:   false,
		},
		{
			name:   "empty output",
			output: "",
			want:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := provider.IsUvManagedInstall(tc.output); got != tc.want {
				t.Errorf("IsUvManagedInstall(%q) = %v, want %v", tc.output, got, tc.want)
			}
		})
	}
}
