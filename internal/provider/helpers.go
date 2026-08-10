package provider

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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

// RunCommandInteractive is like RunCommandWithLog, but also connects the
// command's stdin to the calling process's stdin. RunCommand and friends
// leave Stdin unset, which os/exec connects to /dev/null — fine for
// non-interactive tools, but it means a child process needing to prompt for
// input (e.g. `sudo` asking for a password) has no tty to prompt on and
// fails immediately. Used for commands that may need to prompt interactively
// when run from a real terminal.
func RunCommandInteractive(ctx context.Context, logger *logging.Logger, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
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

// streamWriter is an io.Writer that buffers partial writes and invokes onLine
// for each complete line as it becomes available, so a caller can react to a
// command's progress output while the command is still running.
type streamWriter struct {
	buf    bytes.Buffer
	onLine func(string)
}

func (w *streamWriter) Write(p []byte) (int, error) {
	w.buf.Write(p)
	for {
		data := w.buf.Bytes()
		idx := bytes.IndexByte(data, '\n')
		if idx < 0 {
			break
		}
		line := string(data[:idx])
		w.buf.Next(idx + 1)
		w.onLine(line)
	}
	return len(p), nil
}

// runStreaming is the shared implementation behind RunCommandStreamWithLog and
// RunCommandEnvStreamWithLog.
func runStreaming(ctx context.Context, logger *logging.Logger, envPairs []string, onLine func(string), name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
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
	if onLine != nil {
		writers = append(writers, &streamWriter{onLine: onLine})
	}

	combined := io.MultiWriter(writers...)
	cmd.Stdout = combined
	cmd.Stderr = combined

	err := cmd.Run()
	return buf.String(), err
}

// RunCommandStreamWithLog is like RunCommandWithLog, but also invokes onLine
// for each complete line of combined output as it is produced, before the
// command exits. Used to surface live per-item progress from commands that
// print structured progress lines (e.g. Homebrew's "==> Upgrading <name>").
func RunCommandStreamWithLog(ctx context.Context, logger *logging.Logger, onLine func(string), name string, args ...string) (string, error) {
	return runStreaming(ctx, logger, nil, onLine, name, args...)
}

// RunCommandEnvStreamWithLog is RunCommandStreamWithLog with extra environment variables.
func RunCommandEnvStreamWithLog(ctx context.Context, logger *logging.Logger, envPairs []string, onLine func(string), name string, args ...string) (string, error) {
	return runStreaming(ctx, logger, envPairs, onLine, name, args...)
}

// OnUpgradeProgressLine returns an onLine callback for RunCommandStreamWithLog
// / RunCommandEnvStreamWithLog that reports PackageStarting when line is one
// of Homebrew's own "==> Upgrading <name>" progress markers for a name in
// tracked. Used to surface real per-package progress from a single batched
// `brew upgrade` invocation without running one process per package; names
// not in tracked (Homebrew's own summary header, auto-upgraded dependents)
// are ignored.
func OnUpgradeProgressLine(ctx context.Context, tracked map[string]bool) func(string) {
	return func(line string) {
		if pkg, ok := strings.CutPrefix(line, "==> Upgrading "); ok && tracked[pkg] {
			ReportProgress(ctx, pkg, PackageStarting)
		}
	}
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
// single name. Both return combined output and an error. For the single-name
// and per-item fallback paths, PackageStarting is reported immediately before
// each invocation since it corresponds to a real, individually-running
// command. For the batched fast path, batch() is responsible for reporting
// PackageStarting itself as it observes real per-package progress (e.g. by
// parsing the command's live output) — BatchUpgrade only reports the
// terminal PackageUpdated once the whole batch succeeds. Returns the updated
// and failed name lists.
func BatchUpgrade(
	ctx context.Context,
	names []string,
	batch func(ctx context.Context, names []string) (string, error),
	one func(ctx context.Context, name string) (string, error),
) (updated, failed []string) {
	if len(names) == 0 {
		return nil, nil
	}

	// A single package gains nothing from batching; run it directly so a
	// failure isn't paid for twice.
	if len(names) == 1 {
		ReportProgress(ctx, names[0], PackageStarting)
		if _, err := one(ctx, names[0]); err != nil {
			ReportProgress(ctx, names[0], PackageFailed)
			return nil, []string{names[0]}
		}
		ReportProgress(ctx, names[0], PackageUpdated)
		return []string{names[0]}, nil
	}

	// Fast path: one batched command for everything. batch() reports
	// PackageStarting per name itself as real progress is observed.
	if _, err := batch(ctx, names); err == nil {
		for _, n := range names {
			ReportProgress(ctx, n, PackageUpdated)
		}
		return append([]string(nil), names...), nil
	}

	// Batch failed somewhere; re-run each to attribute outcomes precisely.
	for _, n := range names {
		ReportProgress(ctx, n, PackageStarting)
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
