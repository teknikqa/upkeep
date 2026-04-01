package errors_test

import (
	"errors"
	"fmt"
	"testing"

	upkeeperrors "github.com/teknikqa/upkeep/internal/errors"
)

// --- ProviderError ---

func TestProviderError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *upkeeperrors.ProviderError
		wantMsg string
	}{
		{
			name:    "with wrapped error",
			err:     &upkeeperrors.ProviderError{Provider: "brew", Phase: "scan", Err: fmt.Errorf("something failed")},
			wantMsg: `provider "brew" scan: something failed`,
		},
		{
			name:    "update phase",
			err:     &upkeeperrors.ProviderError{Provider: "npm", Phase: "update", Err: fmt.Errorf("update failed")},
			wantMsg: `provider "npm" update: update failed`,
		},
		{
			name:    "nil underlying error",
			err:     &upkeeperrors.ProviderError{Provider: "pip", Phase: "scan", Err: nil},
			wantMsg: `provider "pip" scan: <nil>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("ProviderError.Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestProviderError_Unwrap(t *testing.T) {
	sentinel := fmt.Errorf("sentinel error")
	pe := &upkeeperrors.ProviderError{Provider: "brew", Phase: "scan", Err: sentinel}

	if !errors.Is(pe, sentinel) {
		t.Error("errors.Is() should find the sentinel via Unwrap")
	}

	var target *upkeeperrors.ProviderError
	wrapped := fmt.Errorf("outer: %w", pe)
	if !errors.As(wrapped, &target) {
		t.Error("errors.As() should find ProviderError through outer wrapping")
	}
	if target.Provider != "brew" {
		t.Errorf("errors.As() provider = %q, want %q", target.Provider, "brew")
	}
}

func TestProviderError_NilErr_NoNilPanic(t *testing.T) {
	pe := &upkeeperrors.ProviderError{Provider: "rust", Phase: "update", Err: nil}
	// Calling Error() with nil Err must not panic.
	_ = pe.Error()
	// Unwrap should return nil.
	if pe.Unwrap() != nil {
		t.Error("Unwrap() of nil Err should return nil")
	}
}

// --- ConfigError ---

func TestConfigError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *upkeeperrors.ConfigError
		wantMsg string
	}{
		{
			name:    "with field and error",
			err:     &upkeeperrors.ConfigError{Field: "providers.brew.enabled", Err: fmt.Errorf("must be bool")},
			wantMsg: `config field "providers.brew.enabled": must be bool`,
		},
		{
			name:    "nil underlying error",
			err:     &upkeeperrors.ConfigError{Field: "parallelism", Err: nil},
			wantMsg: `config field "parallelism": <nil>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("ConfigError.Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestConfigError_Unwrap(t *testing.T) {
	sentinel := fmt.Errorf("invalid value")
	ce := &upkeeperrors.ConfigError{Field: "logging.level", Err: sentinel}

	if !errors.Is(ce, sentinel) {
		t.Error("errors.Is() should find the sentinel via Unwrap")
	}

	var target *upkeeperrors.ConfigError
	wrapped := fmt.Errorf("config: %w", ce)
	if !errors.As(wrapped, &target) {
		t.Error("errors.As() should find ConfigError through outer wrapping")
	}
	if target.Field != "logging.level" {
		t.Errorf("errors.As() field = %q, want %q", target.Field, "logging.level")
	}
}

func TestConfigError_NilErr_NoNilPanic(t *testing.T) {
	ce := &upkeeperrors.ConfigError{Field: "parallelism", Err: nil}
	_ = ce.Error()
	if ce.Unwrap() != nil {
		t.Error("Unwrap() of nil Err should return nil")
	}
}

// --- StateCorruptedError ---

func TestStateCorruptedError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *upkeeperrors.StateCorruptedError
		wantMsg string
	}{
		{
			name:    "with path and error",
			err:     &upkeeperrors.StateCorruptedError{Path: "/home/user/.local/state/upkeep/last-run.json", Err: fmt.Errorf("invalid JSON")},
			wantMsg: `state file "/home/user/.local/state/upkeep/last-run.json": invalid JSON`,
		},
		{
			name:    "nil underlying error",
			err:     &upkeeperrors.StateCorruptedError{Path: "/tmp/state.json", Err: nil},
			wantMsg: `state file "/tmp/state.json": <nil>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("StateCorruptedError.Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestStateCorruptedError_Unwrap(t *testing.T) {
	sentinel := fmt.Errorf("unexpected EOF")
	sce := &upkeeperrors.StateCorruptedError{Path: "/tmp/state.json", Err: sentinel}

	if !errors.Is(sce, sentinel) {
		t.Error("errors.Is() should find the sentinel via Unwrap")
	}

	var target *upkeeperrors.StateCorruptedError
	wrapped := fmt.Errorf("load: %w", sce)
	if !errors.As(wrapped, &target) {
		t.Error("errors.As() should find StateCorruptedError through outer wrapping")
	}
	if target.Path != "/tmp/state.json" {
		t.Errorf("errors.As() path = %q, want %q", target.Path, "/tmp/state.json")
	}
}

func TestStateCorruptedError_NilErr_NoNilPanic(t *testing.T) {
	sce := &upkeeperrors.StateCorruptedError{Path: "/tmp/state.json", Err: nil}
	_ = sce.Error()
	if sce.Unwrap() != nil {
		t.Error("Unwrap() of nil Err should return nil")
	}
}
