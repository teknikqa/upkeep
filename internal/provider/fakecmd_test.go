package provider_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teknikqa/upkeep/internal/provider"
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

// fakeResponse is a fake command's canned reply to one argument list.
type fakeResponse struct {
	stdout, stderr string
	exit           int
}

// isolatedPath points PATH at a fresh empty directory, so only fakes added
// with fakeCommand are found (never the developer's real brew or cargo).
func isolatedPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	return dir
}

// fakeCommand writes an executable name into dir that answers each argument
// list in responses (e.g. "outdated --json=v2") with its canned reply. The
// "*" key answers any other arguments; without it they exit 127.
func fakeCommand(t *testing.T, dir, name string, responses map[string]fakeResponse) {
	t.Helper()
	quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
	write := func(path, content string) {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var cases []string
	fallback := "exit 127"
	i := 0
	for args, r := range responses {
		out := filepath.Join(dir, fmt.Sprintf("%s.%d.out", name, i))
		errOut := filepath.Join(dir, fmt.Sprintf("%s.%d.err", name, i))
		write(out, r.stdout)
		write(errOut, r.stderr)
		reply := fmt.Sprintf("/bin/cat %s; /bin/cat %s >&2; exit %d", quote(out), quote(errOut), r.exit)
		if args == "*" {
			fallback = reply
		} else {
			cases = append(cases, fmt.Sprintf("  %s) %s ;;", quote(args), reply))
		}
		i++
	}
	script := fmt.Sprintf("#!/bin/sh\ncase \"$*\" in\n%s\n  *) %s ;;\nesac\n", strings.Join(cases, "\n"), fallback)
	if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
}

// itemNames returns the names of items, for compact assertions.
func itemNames(items []provider.OutdatedItem) []string {
	names := make([]string, len(items))
	for i, it := range items {
		names[i] = it.Name
	}
	return names
}
