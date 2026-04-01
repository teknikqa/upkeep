// Package errors defines structured error types for upkeep operations.
// These types wrap underlying errors with additional context (provider name,
// config field, state file path) and support errors.Is()/errors.As() unwrapping.
package errors

import "fmt"

// ProviderError wraps an error from a provider Scan or Update operation,
// annotating it with the provider name and the phase ("scan" or "update")
// in which the error occurred.
type ProviderError struct {
	// Provider is the name of the provider (e.g., "brew", "npm").
	Provider string
	// Phase is the operation phase: "scan" or "update".
	Phase string
	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *ProviderError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("provider %q %s: <nil>", e.Provider, e.Phase)
	}
	return fmt.Sprintf("provider %q %s: %s", e.Provider, e.Phase, e.Err.Error())
}

// Unwrap returns the underlying error, enabling errors.Is()/errors.As() unwrapping.
func (e *ProviderError) Unwrap() error { return e.Err }

// ConfigError wraps a configuration validation or load error, annotating it
// with the field path where the error was detected (e.g., "providers.brew.enabled").
type ConfigError struct {
	// Field is the dotted path to the offending config field.
	Field string
	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *ConfigError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("config field %q: <nil>", e.Field)
	}
	return fmt.Sprintf("config field %q: %s", e.Field, e.Err.Error())
}

// Unwrap returns the underlying error, enabling errors.Is()/errors.As() unwrapping.
func (e *ConfigError) Unwrap() error { return e.Err }

// StateCorruptedError wraps a state file parse or lock error, annotating it
// with the file path of the corrupted state file.
type StateCorruptedError struct {
	// Path is the file path of the state file.
	Path string
	// Err is the underlying error.
	Err error
}

// Error implements the error interface.
func (e *StateCorruptedError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("state file %q: <nil>", e.Path)
	}
	return fmt.Sprintf("state file %q: %s", e.Path, e.Err.Error())
}

// Unwrap returns the underlying error, enabling errors.Is()/errors.As() unwrapping.
func (e *StateCorruptedError) Unwrap() error { return e.Err }
