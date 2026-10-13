package cmd

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// isolateHome points HOME at a temp dir so the default config, state and log
// paths never touch the real user's files.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

// captureStdout runs fn and returns what it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()
	w.Close()
	out, _ := io.ReadAll(r)
	return string(out)
}

func cmdWithContext() *cobra.Command {
	c := &cobra.Command{}
	c.SetContext(context.Background())
	return c
}

// A provider filter that matches nothing exercises the whole setup path
// (config, logger, state, engine) without scanning the real system.
func TestRunUpdate_NoMatchingProviders(t *testing.T) {
	isolateHome(t)
	oldForce := forceInteractive
	forceInteractive = true
	t.Cleanup(func() { forceInteractive = oldForce })

	var err error
	out := captureStdout(t, func() {
		err = runUpdate(cmdWithContext(), []string{"no-such-provider"})
	})
	if err != nil {
		t.Fatalf("runUpdate: %v", err)
	}
	if !strings.Contains(out, "No providers to run.") {
		t.Errorf("expected %q in output, got %q", "No providers to run.", out)
	}
}

func TestRunUpdate_InvalidConfigReturnsError(t *testing.T) {
	isolateHome(t)
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("parallelism: [not, a, number\n"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}
	old := cfgFile
	cfgFile = path
	t.Cleanup(func() { cfgFile = old })

	err := runUpdate(cmdWithContext(), nil)
	if err == nil || !strings.Contains(err.Error(), "loading config") {
		t.Fatalf("expected loading config error, got %v", err)
	}
}

func TestRunUpdate_CorruptStateIsNotFatal(t *testing.T) {
	home := isolateHome(t)
	statePath := filepath.Join(home, ".local", "state", "upkeep", "last-run.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0o750); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if err := os.WriteFile(statePath, []byte("{not json"), 0o600); err != nil {
		t.Fatalf("setup: %v", err)
	}

	var err error
	out := captureStdout(t, func() {
		err = runUpdate(cmdWithContext(), []string{"no-such-provider"})
	})
	if err != nil {
		t.Fatalf("corrupt state should be non-fatal, got %v", err)
	}
	if !strings.Contains(out, "No providers to run.") {
		t.Errorf("expected run to continue, got output %q", out)
	}
	logs, _ := filepath.Glob(filepath.Join(home, "Library", "Logs", "upkeep", "*"))
	var logged string
	for _, f := range logs {
		b, _ := os.ReadFile(f)
		logged += string(b)
	}
	if !strings.Contains(logged, "loading state") {
		t.Errorf("expected a 'loading state' warning in the log under temp HOME, got %q", logged)
	}
}

// Positional args reach runUpdate through the root command's RunE.
func TestExecute_RunsUpdateWithProviderArgs(t *testing.T) {
	isolateHome(t)
	rootCmd.SetArgs([]string{"no-such-provider"})
	t.Cleanup(func() { rootCmd.SetArgs(nil) })

	var err error
	out := captureStdout(t, func() { err = execute() })
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !strings.Contains(out, "No providers to run.") {
		t.Errorf("expected %q in output, got %q", "No providers to run.", out)
	}
}
