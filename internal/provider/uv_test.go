package provider_test

import (
	"context"
	"errors"
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

func TestUvProvider_Scan_NotAvailable(t *testing.T) {
	p := provider.NewUvProvider(config.UvConfig{Enabled: true, SelfUpdate: true, Tool: true}, nil)
	p.SetCheckAvailable(func() bool { return false })

	result := p.Scan(context.Background())
	if result.Available {
		t.Error("expected Available=false")
	}
	if result.Message == "" {
		t.Error("expected a non-empty message")
	}
}

func TestUvProvider_Scan_Available(t *testing.T) {
	tests := []struct {
		name       string
		selfUpdate bool
		tool       bool
		wantNames  []string
	}{
		{"both enabled", true, true, []string{"uv (self)", "uv tool (all packages)"}},
		{"self-update only", true, false, []string{"uv (self)"}},
		{"tool only", false, true, []string{"uv tool (all packages)"}},
		{"both disabled", false, false, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := provider.NewUvProvider(config.UvConfig{Enabled: true, SelfUpdate: tt.selfUpdate, Tool: tt.tool}, nil)
			p.SetCheckAvailable(func() bool { return true })

			result := p.Scan(context.Background())
			if !result.Available {
				t.Fatal("expected Available=true")
			}
			if len(result.Outdated) != len(tt.wantNames) {
				t.Fatalf("expected %d items, got %d: %+v", len(tt.wantNames), len(result.Outdated), result.Outdated)
			}
			for i, name := range tt.wantNames {
				if result.Outdated[i].Name != name {
					t.Errorf("item %d: expected %q, got %q", i, name, result.Outdated[i].Name)
				}
			}
		})
	}
}

func TestUvProvider_Update_NotAvailable(t *testing.T) {
	p := provider.NewUvProvider(config.UvConfig{Enabled: true, SelfUpdate: true, Tool: true}, nil)
	p.SetCheckAvailable(func() bool { return false })
	p.SetRunSelfUpdate(func(_ context.Context) (string, error) {
		t.Fatal("self update should not run when uv is unavailable")
		return "", nil
	})
	p.SetRunToolUpgrade(func(_ context.Context) (string, error) {
		t.Fatal("tool upgrade should not run when uv is unavailable")
		return "", nil
	})

	result := p.Update(context.Background(), nil)
	if len(result.Updated) != 0 || len(result.Failed) != 0 || len(result.Skipped) != 0 {
		t.Errorf("expected empty result, got %+v", result)
	}
}

func TestUvProvider_Update_SelfUpdate(t *testing.T) {
	tests := []struct {
		name        string
		out         string
		err         error
		wantUpdated bool
		wantSkipped bool
		wantFailed  bool
	}{
		{
			name:        "success",
			out:         "",
			err:         nil,
			wantUpdated: true,
		},
		{
			name:        "managed install is skipped",
			out:         "error: uv was installed through an external package manager and cannot update itself.",
			err:         errors.New("exit status 2"),
			wantSkipped: true,
		},
		{
			name:       "other error fails",
			out:        "error: network connection failed",
			err:        errors.New("exit status 1"),
			wantFailed: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := provider.NewUvProvider(config.UvConfig{Enabled: true, SelfUpdate: true, Tool: false}, nil)
			p.SetCheckAvailable(func() bool { return true })
			p.SetRunSelfUpdate(func(_ context.Context) (string, error) {
				return tt.out, tt.err
			})

			result := p.Update(context.Background(), nil)

			gotUpdated := contains(result.Updated, "uv-self")
			gotSkipped := contains(result.Skipped, "uv-self")
			gotFailed := contains(result.Failed, "uv-self")

			if gotUpdated != tt.wantUpdated {
				t.Errorf("updated: got %v, want %v (result=%+v)", gotUpdated, tt.wantUpdated, result)
			}
			if gotSkipped != tt.wantSkipped {
				t.Errorf("skipped: got %v, want %v (result=%+v)", gotSkipped, tt.wantSkipped, result)
			}
			if gotFailed != tt.wantFailed {
				t.Errorf("failed: got %v, want %v (result=%+v)", gotFailed, tt.wantFailed, result)
			}
		})
	}
}

func TestUvProvider_Update_ToolUpgrade(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantUpdated bool
		wantFailed  bool
	}{
		{name: "success", err: nil, wantUpdated: true},
		{name: "failure", err: errors.New("upgrade failed"), wantFailed: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := provider.NewUvProvider(config.UvConfig{Enabled: true, SelfUpdate: false, Tool: true}, nil)
			p.SetCheckAvailable(func() bool { return true })
			p.SetRunToolUpgrade(func(_ context.Context) (string, error) {
				return "", tt.err
			})

			result := p.Update(context.Background(), nil)

			gotUpdated := contains(result.Updated, "uv-tools")
			gotFailed := contains(result.Failed, "uv-tools")

			if gotUpdated != tt.wantUpdated {
				t.Errorf("updated: got %v, want %v (result=%+v)", gotUpdated, tt.wantUpdated, result)
			}
			if gotFailed != tt.wantFailed {
				t.Errorf("failed: got %v, want %v (result=%+v)", gotFailed, tt.wantFailed, result)
			}
		})
	}
}

func TestUvProvider_Update_BothDisabled(t *testing.T) {
	p := provider.NewUvProvider(config.UvConfig{Enabled: true, SelfUpdate: false, Tool: false}, nil)
	p.SetCheckAvailable(func() bool { return true })

	result := p.Update(context.Background(), nil)
	if len(result.Updated) != 0 || len(result.Failed) != 0 || len(result.Skipped) != 0 {
		t.Errorf("expected empty result, got %+v", result)
	}
}

func contains(names []string, target string) bool {
	for _, n := range names {
		if n == target {
			return true
		}
	}
	return false
}
