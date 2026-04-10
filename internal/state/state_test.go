package state_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/teknikqa/upkeep/internal/state"
)

func TestSaveLoad_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state", "last-run.json")

	s := state.New(path)
	s.LastRun = time.Date(2026, 3, 30, 10, 0, 0, 0, time.UTC)
	s.DurationSeconds = 120.5
	s.SetProviderResult("brew", state.ProviderStatus{
		Status:  "success",
		Updated: []string{"git", "jq"},
	})
	s.SetProviderResult("npm", state.ProviderStatus{
		Status: "failed",
		Failed: []string{"some-pkg"},
	})
	s.Deferred = state.DeferredState{
		Casks:  []string{"docker"},
		Script: "~/.local/state/upkeep/deferred-cask.sh",
	}

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded.DurationSeconds != 120.5 {
		t.Errorf("expected DurationSeconds=120.5, got %f", loaded.DurationSeconds)
	}
	if len(loaded.Providers) != 2 {
		t.Errorf("expected 2 providers, got %d", len(loaded.Providers))
	}
	if loaded.Providers["brew"].Status != "success" {
		t.Errorf("expected brew status=success, got %q", loaded.Providers["brew"].Status)
	}
	if len(loaded.Deferred.Casks) != 1 || loaded.Deferred.Casks[0] != "docker" {
		t.Errorf("expected deferred casks=[docker], got %v", loaded.Deferred.Casks)
	}
}

func TestLoad_MissingFileReturnsEmpty(t *testing.T) {
	s, err := state.Load("/nonexistent/path/last-run.json")
	if err != nil {
		t.Fatalf("expected nil error for missing file, got: %v", err)
	}
	if s == nil {
		t.Fatal("expected non-nil state")
	}
	if len(s.Providers) != 0 {
		t.Errorf("expected empty providers, got %d", len(s.Providers))
	}
}

func TestGetFailed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "last-run.json")

	s := state.New(path)
	s.SetProviderResult("brew", state.ProviderStatus{Status: "success"})
	s.SetProviderResult("npm", state.ProviderStatus{Status: "failed"})
	s.SetProviderResult("pip", state.ProviderStatus{Status: "failed"})
	s.SetProviderResult("rust", state.ProviderStatus{Status: "partial"})

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	failed := loaded.GetFailed()
	if len(failed) != 2 {
		t.Errorf("expected 2 failed providers, got %d: %v", len(failed), failed)
	}
	// Ensure npm and pip are in the failed list.
	failedSet := make(map[string]bool)
	for _, f := range failed {
		failedSet[f] = true
	}
	if !failedSet["npm"] || !failedSet["pip"] {
		t.Errorf("expected npm and pip in failed list, got %v", failed)
	}
}

func TestGetDeferred(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "last-run.json")

	s := state.New(path)
	s.Deferred = state.DeferredState{
		Casks:            []string{"docker", "some-auth-cask"},
		Script:           "/some/path/deferred.sh",
		NotificationSent: true,
	}

	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := state.Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	d := loaded.GetDeferred()
	if len(d.Casks) != 2 {
		t.Errorf("expected 2 deferred casks, got %d", len(d.Casks))
	}
	if !d.NotificationSent {
		t.Error("expected notification_sent=true")
	}
}

func TestClear(t *testing.T) {
	s := state.New("")
	s.SetProviderResult("brew", state.ProviderStatus{Status: "success"})
	s.DurationSeconds = 99

	s.Clear()

	if len(s.Providers) != 0 {
		t.Errorf("expected empty providers after Clear, got %d", len(s.Providers))
	}
	if s.DurationSeconds != 0 {
		t.Errorf("expected DurationSeconds=0 after Clear, got %f", s.DurationSeconds)
	}
}

func TestSave_CreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	// Use a nested path that doesn't exist yet.
	path := filepath.Join(dir, "nested", "deep", "last-run.json")

	s := state.New(path)
	if err := s.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected state file to exist at %q: %v", path, err)
	}
}

// --- expandHome / DefaultStatePath ---

func TestExpandHome_WithTilde(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	got := state.ExportExpandHome("~/foo/bar")
	want := filepath.Join(home, "foo/bar")
	if got != want {
		t.Errorf("expandHome(\"~/foo/bar\") = %q, want %q", got, want)
	}
}

func TestExpandHome_WithoutTilde(t *testing.T) {
	got := state.ExportExpandHome("/absolute/path")
	if got != "/absolute/path" {
		t.Errorf("expandHome(\"/absolute/path\") = %q, want unchanged", got)
	}
}

func TestExpandHome_EmptyString(t *testing.T) {
	got := state.ExportExpandHome("")
	if got != "" {
		t.Errorf("expandHome(\"\") = %q, want \"\"", got)
	}
}

func TestExpandHome_TildeOnly(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	got := state.ExportExpandHome("~")
	// filepath.Join(home, "") = home
	if got != home {
		t.Errorf("expandHome(\"~\") = %q, want %q", got, home)
	}
}

func TestDefaultStatePath_ContainsHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot determine home dir")
	}
	got := state.DefaultStatePath()
	if !filepath.IsAbs(got) {
		t.Errorf("DefaultStatePath() = %q should be absolute", got)
	}
	if len(got) < len(home) || got[:len(home)] != home {
		t.Errorf("DefaultStatePath() = %q should start with home dir %q", got, home)
	}
	if filepath.Base(got) != "last-run.json" {
		t.Errorf("DefaultStatePath() = %q should end with last-run.json", got)
	}
}

// TestSave_ReadOnlyDir verifies that Save fails gracefully when the parent
// directory cannot be created or written to.
func TestSave_ReadOnlyDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere — skipping read-only dir test")
	}
	// /dev/null is a file, not a directory, so creating a child under it fails.
	s := state.New("/dev/null/impossible/last-run.json")
	err := s.Save()
	if err == nil {
		t.Fatal("expected error saving to /dev/null/impossible, got nil")
	}
}
