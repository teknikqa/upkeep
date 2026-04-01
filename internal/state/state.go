// Package state manages the upkeep run state file for resumability.
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	upkeeperrors "github.com/teknikqa/upkeep/internal/errors"
	"golang.org/x/sys/unix"
)

const (
	// StateVersion is the current state file schema version.
	StateVersion = 1

	defaultStatePath = "~/.local/state/upkeep/last-run.json"
)

// DefaultStatePath returns the default state file path.
func DefaultStatePath() string {
	return expandHome(defaultStatePath)
}

// ProviderStatus represents the result status of a single provider.
type ProviderStatus struct {
	Status          string    `json:"status"` // "success" | "partial" | "failed" | "skipped"
	Updated         []string  `json:"updated"`
	Failed          []string  `json:"failed"`
	Deferred        []string  `json:"deferred"`
	Skipped         []string  `json:"skipped"`
	DurationSeconds float64   `json:"duration_seconds"`
	Error           *string   `json:"error"`
	Timestamp       time.Time `json:"timestamp"`
}

// DeferredState holds information about deferred cask updates.
type DeferredState struct {
	Casks            []string `json:"casks"`
	Script           string   `json:"script"`
	NotificationSent bool     `json:"notification_sent"`
}

// State is the top-level run state structure.
type State struct {
	Version         int                       `json:"version"`
	LastRun         time.Time                 `json:"last_run"`
	DurationSeconds float64                   `json:"duration_seconds"`
	Providers       map[string]ProviderStatus `json:"providers"`
	Deferred        DeferredState             `json:"deferred"`

	// path is the file path used for Save/Load (not serialised).
	path string
}

// New returns an empty State bound to the given path.
func New(path string) *State {
	if path == "" {
		path = DefaultStatePath()
	}
	return &State{
		Version:   StateVersion,
		Providers: make(map[string]ProviderStatus),
		path:      path,
	}
}

// Load reads the state file from path and returns the State.
// If the file does not exist, an empty State is returned.
func Load(path string) (*State, error) {
	if path == "" {
		path = DefaultStatePath()
	}
	s := New(path)

	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return nil, fmt.Errorf("opening state file %q: %w", path, err)
	}
	defer f.Close()

	// Acquire shared lock for reading.
	if err := lockFile(f, false); err != nil {
		return nil, fmt.Errorf("locking state file for read: %w", err)
	}
	defer unlockFile(f) //nolint:errcheck

	if err := json.NewDecoder(f).Decode(s); err != nil {
		return nil, &upkeeperrors.StateCorruptedError{Path: path, Err: err}
	}
	s.path = path
	return s, nil
}

// Save writes the current state to the state file atomically.
// It creates the parent directory if it doesn't exist.
func (s *State) Save() error {
	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("creating state directory %q: %w", dir, err)
	}

	// Write to a temp file then rename for atomic replacement.
	tmp, err := os.CreateTemp(dir, ".upkeep-state-*.json")
	if err != nil {
		return fmt.Errorf("creating temp state file: %w", err)
	}
	tmpName := tmp.Name()

	// Acquire exclusive lock on the temp file while writing.
	if err := lockFile(tmp, true); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("locking temp state file: %w", err)
	}

	enc := json.NewEncoder(tmp)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s); err != nil {
		unlockFile(tmp) //nolint:errcheck
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("encoding state: %w", err)
	}

	unlockFile(tmp) //nolint:errcheck
	tmp.Close()

	if err := os.Rename(tmpName, s.path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming temp state file to %q: %w", s.path, err)
	}
	return nil
}

// GetFailed returns the names of providers that have status "failed".
func (s *State) GetFailed() []string {
	var failed []string
	for name, ps := range s.Providers {
		if ps.Status == "failed" {
			failed = append(failed, name)
		}
	}
	return failed
}

// GetDeferred returns the deferred cask state from the last run.
func (s *State) GetDeferred() DeferredState {
	return s.Deferred
}

// Clear resets the state to empty (keeps the path).
func (s *State) Clear() {
	s.Version = StateVersion
	s.LastRun = time.Time{}
	s.DurationSeconds = 0
	s.Providers = make(map[string]ProviderStatus)
	s.Deferred = DeferredState{}
}

// SetProviderResult records the result for a provider.
func (s *State) SetProviderResult(name string, ps ProviderStatus) {
	if s.Providers == nil {
		s.Providers = make(map[string]ProviderStatus)
	}
	s.Providers[name] = ps
}

// lockFile acquires an advisory flock on f.
// exclusive=true acquires an exclusive (write) lock; false acquires a shared (read) lock.
// The flock is advisory — it only blocks other processes (or goroutines) that also call
// lockFile. Save uses a temp+rename pattern so readers never see a partial write: the
// exclusive lock on the temp file prevents concurrent writers from racing, and os.Rename
// is atomic on POSIX filesystems.
func lockFile(f *os.File, exclusive bool) error {
	how := unix.LOCK_SH
	if exclusive {
		how = unix.LOCK_EX
	}
	return unix.Flock(int(f.Fd()), how)
}

// unlockFile releases the flock on f.
func unlockFile(f *os.File) error {
	return unix.Flock(int(f.Fd()), unix.LOCK_UN)
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if len(path) == 0 || path[0] != '~' {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[1:])
}
