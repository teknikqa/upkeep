package provider_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/teknikqa/upkeep/internal/logging"
	"github.com/teknikqa/upkeep/internal/provider"
)

func TestCommandExists_KnownCommand(t *testing.T) {
	// 'ls' exists on all Unix systems.
	if !provider.CommandExists("ls") {
		t.Error("expected CommandExists('ls') = true")
	}
}

func TestCommandExists_UnknownCommand(t *testing.T) {
	if provider.CommandExists("nonexistent-xyz-command-42") {
		t.Error("expected CommandExists('nonexistent-xyz-command-42') = false")
	}
}

func TestRunCommand_SimpleCommand(t *testing.T) {
	stdout, stderr, err := provider.RunCommand(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("RunCommand: %v", err)
	}
	if stderr != "" {
		t.Errorf("expected empty stderr, got %q", stderr)
	}
	// echo adds a newline.
	if stdout != "hello\n" {
		t.Errorf("expected stdout='hello\\n', got %q", stdout)
	}
}

func TestRunCommand_TimeoutCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// sleep 10 should be cancelled well before completion.
	_, _, err := provider.RunCommand(ctx, "sleep", "10")
	if err == nil {
		t.Fatal("expected error from cancelled context, got nil")
	}
}

func TestRunCommand_NonZeroExit(t *testing.T) {
	_, _, err := provider.RunCommand(context.Background(), "false")
	if err == nil {
		t.Fatal("expected error from non-zero exit, got nil")
	}
}

// --- ExitCode ---

func TestExitCode_Nil(t *testing.T) {
	if code := provider.ExitCode(nil); code != 0 {
		t.Errorf("expected 0, got %d", code)
	}
}

func TestExitCode_ExitError(t *testing.T) {
	// Run a command that exits with code 42.
	cmd := exec.Command("sh", "-c", "exit 42")
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit from sh -c 'exit 42'")
	}
	code := provider.ExitCode(err)
	if code != 42 {
		t.Errorf("expected exit code 42, got %d", code)
	}
}

func TestExitCode_OtherError(t *testing.T) {
	// A plain non-ExitError should return -1.
	err := context.DeadlineExceeded
	if code := provider.ExitCode(err); code != -1 {
		t.Errorf("expected -1 for non-ExitError, got %d", code)
	}
}

// --- FormatCommand ---

func TestFormatCommand_NoArgs(t *testing.T) {
	result := provider.FormatCommand("echo")
	if !strings.Contains(result, "echo") {
		t.Errorf("expected result to contain 'echo', got %q", result)
	}
}

func TestFormatCommand_WithArgs(t *testing.T) {
	result := provider.FormatCommand("git", "status", "--short")
	if !strings.Contains(result, "git") || !strings.Contains(result, "status") || !strings.Contains(result, "--short") {
		t.Errorf("expected result to contain all args, got %q", result)
	}
}

// --- SetVerboseOutput / getVerboseWriter (via SetVerboseOutput) ---

func TestSetVerboseOutput_SetAndGet(t *testing.T) {
	var buf bytes.Buffer
	provider.SetVerboseOutput(&buf)
	t.Cleanup(func() { provider.SetVerboseOutput(nil) })

	// Verify the verbose writer was picked up by RunCommandWithLog.
	out, err := provider.RunCommandWithLog(context.Background(), nil, "echo", "verbose-test")
	if err != nil {
		t.Fatalf("RunCommandWithLog: %v", err)
	}
	if !strings.Contains(out, "verbose-test") {
		t.Errorf("expected stdout to contain 'verbose-test', got %q", out)
	}
	if !strings.Contains(buf.String(), "verbose-test") {
		t.Errorf("expected verbose buffer to contain 'verbose-test', got %q", buf.String())
	}
}

func TestSetVerboseOutput_NilDisables(t *testing.T) {
	var buf bytes.Buffer
	provider.SetVerboseOutput(&buf)
	provider.SetVerboseOutput(nil) // disable

	_, err := provider.RunCommandWithLog(context.Background(), nil, "echo", "should-not-appear")
	if err != nil {
		t.Fatalf("RunCommandWithLog: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("expected empty verbose buffer after nil, got %q", buf.String())
	}
}

func TestSetVerboseOutput_ConcurrentSafe(t *testing.T) {
	// Exercise the RWMutex under the race detector.
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var b bytes.Buffer
			provider.SetVerboseOutput(&b)
			provider.SetVerboseOutput(nil)
		}()
	}
	wg.Wait()
}

// --- RunCommandEnv ---

func TestRunCommandEnv_InheritsEnv(t *testing.T) {
	// Set a custom env var and verify the command can see it.
	stdout, _, err := provider.RunCommandEnv(context.Background(), []string{"UPKEEP_TEST_VAR=hello123"}, "sh", "-c", "echo $UPKEEP_TEST_VAR")
	if err != nil {
		t.Fatalf("RunCommandEnv: %v", err)
	}
	if !strings.Contains(stdout, "hello123") {
		t.Errorf("expected stdout to contain 'hello123', got %q", stdout)
	}
}

func TestRunCommandEnv_NoExtraEnv(t *testing.T) {
	stdout, _, err := provider.RunCommandEnv(context.Background(), nil, "echo", "hello")
	if err != nil {
		t.Fatalf("RunCommandEnv(nil env): %v", err)
	}
	if !strings.Contains(stdout, "hello") {
		t.Errorf("expected stdout to contain 'hello', got %q", stdout)
	}
}

// --- RunCommandWithLog ---

func TestRunCommandWithLog_CapturesOutput(t *testing.T) {
	dir := t.TempDir()
	logger := logging.New(dir, logging.LevelInfo)
	defer logger.Close()

	out, err := provider.RunCommandWithLog(context.Background(), logger, "echo", "logged-output")
	if err != nil {
		t.Fatalf("RunCommandWithLog: %v", err)
	}
	if !strings.Contains(out, "logged-output") {
		t.Errorf("expected return value to contain 'logged-output', got %q", out)
	}

	// Verify the log file was written.
	logPath := logger.CurrentLogPath()
	data, readErr := os.ReadFile(logPath)
	if readErr != nil {
		t.Fatalf("reading log file: %v", readErr)
	}
	if !strings.Contains(string(data), "logged-output") {
		t.Errorf("expected log file to contain 'logged-output', got %q", string(data))
	}
}

func TestRunCommandWithLog_NilLogger(t *testing.T) {
	// Should not panic when logger is nil.
	out, err := provider.RunCommandWithLog(context.Background(), nil, "echo", "no-logger")
	if err != nil {
		t.Fatalf("RunCommandWithLog(nil logger): %v", err)
	}
	if !strings.Contains(out, "no-logger") {
		t.Errorf("expected stdout to contain 'no-logger', got %q", out)
	}
}

// --- RunCommandEnvWithLog ---

func TestRunCommandEnvWithLog_CapturesOutput(t *testing.T) {
	dir := t.TempDir()
	logger := logging.New(dir, logging.LevelInfo)
	defer logger.Close()

	out, err := provider.RunCommandEnvWithLog(context.Background(), logger, []string{"UPKEEP_TEST_VAR=env-logged"}, "sh", "-c", "echo $UPKEEP_TEST_VAR")
	if err != nil {
		t.Fatalf("RunCommandEnvWithLog: %v", err)
	}
	if !strings.Contains(out, "env-logged") {
		t.Errorf("expected return value to contain 'env-logged', got %q", out)
	}
}

// --- RunCommandVerbose ---

func TestRunCommandVerbose_TeesOutput(t *testing.T) {
	var extraBuf bytes.Buffer
	out, err := provider.RunCommandVerbose(context.Background(), nil, &extraBuf, "echo", "verbose-tee")
	if err != nil {
		t.Fatalf("RunCommandVerbose: %v", err)
	}
	if !strings.Contains(out, "verbose-tee") {
		t.Errorf("expected return to contain 'verbose-tee', got %q", out)
	}
	if !strings.Contains(extraBuf.String(), "verbose-tee") {
		t.Errorf("expected extraWriter to contain 'verbose-tee', got %q", extraBuf.String())
	}
}

func TestRunCommandVerbose_NilExtraWriter(t *testing.T) {
	// Should not panic with nil extra writer.
	out, err := provider.RunCommandVerbose(context.Background(), nil, nil, "echo", "no-extra-writer")
	if err != nil {
		t.Fatalf("RunCommandVerbose(nil extraWriter): %v", err)
	}
	if !strings.Contains(out, "no-extra-writer") {
		t.Errorf("expected stdout to contain 'no-extra-writer', got %q", out)
	}
}

// --- ProgressFunc context helpers ---

func TestContextWithProgress_RoundTrip(t *testing.T) {
	var received []provider.PackageProgress
	fn := func(p provider.PackageProgress) {
		received = append(received, p)
	}

	ctx := provider.ContextWithProgress(context.Background(), fn)
	got := provider.ProgressFromContext(ctx)
	if got == nil {
		t.Fatal("expected ProgressFunc from context, got nil")
	}

	got(provider.PackageProgress{Name: "pkg1", Status: provider.PackageUpdated})
	if len(received) != 1 || received[0].Name != "pkg1" {
		t.Errorf("expected 1 progress event for pkg1, got %+v", received)
	}
}

func TestProgressFromContext_EmptyContext(t *testing.T) {
	got := provider.ProgressFromContext(context.Background())
	if got != nil {
		t.Error("expected nil ProgressFunc from empty context")
	}
}

func TestReportProgress_WithFunc(t *testing.T) {
	var received []provider.PackageProgress
	fn := func(p provider.PackageProgress) {
		received = append(received, p)
	}
	ctx := provider.ContextWithProgress(context.Background(), fn)

	provider.ReportProgress(ctx, "pkg-a", provider.PackageUpdated)
	provider.ReportProgress(ctx, "pkg-b", provider.PackageFailed)

	if len(received) != 2 {
		t.Fatalf("expected 2 progress events, got %d", len(received))
	}
	if received[0].Status != provider.PackageUpdated {
		t.Errorf("expected PackageUpdated, got %s", received[0].Status)
	}
	if received[1].Status != provider.PackageFailed {
		t.Errorf("expected PackageFailed, got %s", received[1].Status)
	}
}

func TestReportProgress_WithoutFunc(t *testing.T) {
	// Should not panic when no ProgressFunc is set.
	provider.ReportProgress(context.Background(), "pkg", provider.PackageUpdated)
}

// --- BatchUpgrade ---

func TestBatchUpgrade_Empty(t *testing.T) {
	updated, failed := provider.BatchUpgrade(context.Background(), nil,
		func(context.Context, []string) (string, error) {
			t.Fatal("batch should not run for empty input")
			return "", nil
		},
		func(context.Context, string) (string, error) {
			t.Fatal("one should not run for empty input")
			return "", nil
		},
	)
	if len(updated) != 0 || len(failed) != 0 {
		t.Errorf("expected no results, got updated=%v failed=%v", updated, failed)
	}
}

func TestBatchUpgrade_SingleSuccess(t *testing.T) {
	var batchCalls, oneCalls int
	updated, failed := provider.BatchUpgrade(context.Background(), []string{"pkg"},
		func(context.Context, []string) (string, error) { batchCalls++; return "", nil },
		func(context.Context, string) (string, error) { oneCalls++; return "", nil },
	)
	// A single package skips the batch command and runs directly.
	if batchCalls != 0 {
		t.Errorf("expected batch not called for single item, got %d", batchCalls)
	}
	if oneCalls != 1 {
		t.Errorf("expected one called once, got %d", oneCalls)
	}
	if len(updated) != 1 || updated[0] != "pkg" || len(failed) != 0 {
		t.Errorf("expected updated=[pkg], got updated=%v failed=%v", updated, failed)
	}
}

func TestBatchUpgrade_SingleFailure(t *testing.T) {
	updated, failed := provider.BatchUpgrade(context.Background(), []string{"pkg"},
		func(context.Context, []string) (string, error) { return "", errors.New("nope") },
		func(context.Context, string) (string, error) { return "", errors.New("nope") },
	)
	if len(updated) != 0 || len(failed) != 1 || failed[0] != "pkg" {
		t.Errorf("expected failed=[pkg], got updated=%v failed=%v", updated, failed)
	}
}

func TestBatchUpgrade_BatchSuccess(t *testing.T) {
	var oneCalls int
	names := []string{"a", "b", "c"}
	updated, failed := provider.BatchUpgrade(context.Background(), names,
		func(_ context.Context, n []string) (string, error) {
			if len(n) != 3 {
				t.Errorf("expected 3 names in batch, got %v", n)
			}
			return "", nil
		},
		func(context.Context, string) (string, error) { oneCalls++; return "", nil },
	)
	if oneCalls != 0 {
		t.Errorf("expected no per-item calls on batch success, got %d", oneCalls)
	}
	if len(updated) != 3 || len(failed) != 0 {
		t.Errorf("expected all 3 updated, got updated=%v failed=%v", updated, failed)
	}
}

func TestBatchUpgrade_BatchFailureFallsBackPerItem(t *testing.T) {
	// Batch fails, so each package is retried individually; "b" keeps failing.
	var oneCalls int
	updated, failed := provider.BatchUpgrade(context.Background(), []string{"a", "b", "c"},
		func(context.Context, []string) (string, error) { return "", errors.New("batch failed") },
		func(_ context.Context, name string) (string, error) {
			oneCalls++
			if name == "b" {
				return "", errors.New("b failed")
			}
			return "", nil
		},
	)
	if oneCalls != 3 {
		t.Errorf("expected 3 fallback calls, got %d", oneCalls)
	}
	if len(updated) != 2 || updated[0] != "a" || updated[1] != "c" {
		t.Errorf("expected updated=[a c], got %v", updated)
	}
	if len(failed) != 1 || failed[0] != "b" {
		t.Errorf("expected failed=[b], got %v", failed)
	}
}

func TestBatchUpgrade_ReportsStartingThenTerminal(t *testing.T) {
	events := map[string][]provider.PackageStatus{}
	ctx := provider.ContextWithProgress(context.Background(), func(p provider.PackageProgress) {
		events[p.Name] = append(events[p.Name], p.Status)
	})

	provider.BatchUpgrade(ctx, []string{"a", "b"},
		func(context.Context, []string) (string, error) { return "", errors.New("fail") },
		func(_ context.Context, name string) (string, error) {
			if name == "b" {
				return "", errors.New("b fails")
			}
			return "", nil
		},
	)

	// Every package reports Starting first, then a terminal status.
	want := map[string]provider.PackageStatus{
		"a": provider.PackageUpdated,
		"b": provider.PackageFailed,
	}
	for name, terminal := range want {
		got := events[name]
		if len(got) < 2 {
			t.Fatalf("%s: expected at least 2 events, got %v", name, got)
		}
		if got[0] != provider.PackageStarting {
			t.Errorf("%s: expected first event Starting, got %s", name, got[0])
		}
		if got[len(got)-1] != terminal {
			t.Errorf("%s: expected terminal %s, got %s", name, terminal, got[len(got)-1])
		}
	}
}
