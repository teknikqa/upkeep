package provider_test

import (
	"os"
	"path/filepath"
	"testing"
)

// failingCommands puts scripts named names, each exiting 1, at the front of
// PATH, so tests never run the real sudo/port/mas (sudo would otherwise
// prompt on the developer's terminal and hang).
func failingCommands(t *testing.T, names ...string) {
	t.Helper()
	dir := t.TempDir()
	for _, n := range names {
		if err := os.WriteFile(filepath.Join(dir, n), []byte("#!/bin/sh\nexit 1\n"), 0o700); err != nil {
			t.Fatalf("writing fake %s: %v", n, err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}
