package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/teknikqa/upkeep/internal/logging"
)

// verboseWriter is the global extra writer teed when --verbose is active.
// Protected by verboseMu.
var (
	verboseWriter io.Writer
	verboseMu     sync.RWMutex
)

// SetVerboseOutput sets a global extra writer that is teed from all
// RunCommandWithLog and RunCommandEnvWithLog calls.
// Pass nil to disable. Safe for concurrent use.
func SetVerboseOutput(w io.Writer) {
	verboseMu.Lock()
	verboseWriter = w
	verboseMu.Unlock()
}

// getVerboseWriter returns the current verbose writer (nil if unset).
// Uses a read lock (verboseMu) so concurrent RunCommand* calls can read the
// writer without blocking each other; only SetVerboseOutput holds a write lock.
func getVerboseWriter() io.Writer {
	verboseMu.RLock()
	defer verboseMu.RUnlock()
	return verboseWriter
}

// CommandExists returns true if the named command is found on PATH.
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// RunCommand runs a command with arguments, respecting context for cancellation/timeout.
// Returns combined stdout and stderr as separate strings, plus any error.
func RunCommand(ctx context.Context, name string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// RunCommandWithLog runs a command with arguments, tees combined output to the
// provided logger, and returns the combined output and any error.
// If logger is nil, output is not logged.
// When --verbose is active (SetVerboseOutput was called), output is also teed
// to the configured verbose writer.
func RunCommandWithLog(ctx context.Context, logger *logging.Logger, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var buf bytes.Buffer

	writers := []io.Writer{&buf}
	if logger != nil {
		writers = append(writers, logger.Writer())
	}
	if vw := getVerboseWriter(); vw != nil {
		writers = append(writers, vw)
	}

	combined := io.MultiWriter(writers...)
	cmd.Stdout = combined
	cmd.Stderr = combined

	err := cmd.Run()
	return buf.String(), err
}

// RunCommandEnv is like RunCommand but allows setting extra environment variables.
// envPairs should be in "KEY=VALUE" format.
func RunCommandEnv(ctx context.Context, envPairs []string, name string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	if len(envPairs) > 0 {
		cmd.Env = append(os.Environ(), envPairs...)
	}
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// RunCommandEnvWithLog is like RunCommandWithLog but with extra environment variables.
// Also tees to the verbose writer when active.
func RunCommandEnvWithLog(ctx context.Context, logger *logging.Logger, envPairs []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	// Inherit current process environment then append overrides.
	if len(envPairs) > 0 {
		cmd.Env = append(os.Environ(), envPairs...)
	}
	var buf bytes.Buffer

	writers := []io.Writer{&buf}
	if logger != nil {
		writers = append(writers, logger.Writer())
	}
	if vw := getVerboseWriter(); vw != nil {
		writers = append(writers, vw)
	}

	combined := io.MultiWriter(writers...)
	cmd.Stdout = combined
	cmd.Stderr = combined

	err := cmd.Run()
	return buf.String(), err
}

// RunCommandVerbose runs a command and optionally tees output to an extra writer
// (e.g., os.Stdout for --verbose mode) in addition to the logger.
func RunCommandVerbose(ctx context.Context, logger *logging.Logger, extraWriter io.Writer, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var buf bytes.Buffer

	writers := []io.Writer{&buf}
	if logger != nil {
		writers = append(writers, logger.Writer())
	}
	if extraWriter != nil {
		writers = append(writers, extraWriter)
	}

	combined := io.MultiWriter(writers...)
	cmd.Stdout = combined
	cmd.Stderr = combined

	err := cmd.Run()
	return buf.String(), err
}

// BatchUpgrade upgrades a set of named packages using a single batched command
// for speed, falling back to per-package execution to attribute failures
// accurately when the batch reports an error.
//
// Most package managers (brew, npm, pip) accept many package names in one
// invocation and parallelize downloads internally, which is far faster than N
// separate processes — and, in brew's case, avoids the global-lock contention
// that makes truly-concurrent invocations impossible. The trade-off is coarser
// failure attribution: a batch command exits non-zero if any package fails
// without saying which. When that happens we re-run each package individually
// (already-upgraded ones become fast no-ops) so the updated/failed split stays
// exact.
//
// batch runs the combined command for all names; one runs the command for a
// single name. Both return combined output and an error. PackageStarting is
// reported for every name up front, then PackageUpdated / PackageFailed as
// outcomes are known. Returns the updated and failed name lists.
func BatchUpgrade(
	ctx context.Context,
	names []string,
	batch func(ctx context.Context, names []string) (string, error),
	one func(ctx context.Context, name string) (string, error),
) (updated, failed []string) {
	if len(names) == 0 {
		return nil, nil
	}

	for _, n := range names {
		ReportProgress(ctx, n, PackageStarting)
	}

	// A single package gains nothing from batching; run it directly so a
	// failure isn't paid for twice.
	if len(names) == 1 {
		if _, err := one(ctx, names[0]); err != nil {
			ReportProgress(ctx, names[0], PackageFailed)
			return nil, []string{names[0]}
		}
		ReportProgress(ctx, names[0], PackageUpdated)
		return []string{names[0]}, nil
	}

	// Fast path: one batched command for everything.
	if _, err := batch(ctx, names); err == nil {
		for _, n := range names {
			ReportProgress(ctx, n, PackageUpdated)
		}
		return append([]string(nil), names...), nil
	}

	// Batch failed somewhere; re-run each to attribute outcomes precisely.
	for _, n := range names {
		if _, err := one(ctx, n); err != nil {
			failed = append(failed, n)
			ReportProgress(ctx, n, PackageFailed)
		} else {
			updated = append(updated, n)
			ReportProgress(ctx, n, PackageUpdated)
		}
	}
	return updated, failed
}

// ExitCode extracts the exit code from a command error.
// Returns 0 if err is nil, -1 if the exit code cannot be determined.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return -1
}

// FormatCommand returns a human-readable representation of a command.
func FormatCommand(name string, args ...string) string {
	all := append([]string{name}, args...)
	return fmt.Sprintf("%q", all)
}
