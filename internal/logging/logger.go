// Package logging provides daily rotating log files and subprocess output capture.
package logging

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Logger writes timestamped log entries to a daily log file.
type Logger struct {
	dir   string
	level Level
	mu    sync.Mutex
	file  *os.File
	date  string // current log file date (YYYY-MM-DD)
}

// Level represents log verbosity.
type Level int

const (
	LevelDebug Level = iota
	LevelInfo
	LevelWarn
	LevelError
)

// ParseLevel converts a string to a Level. Defaults to LevelInfo.
func ParseLevel(s string) Level {
	switch s {
	case "debug":
		return LevelDebug
	case "warn":
		return LevelWarn
	case "error":
		return LevelError
	default:
		return LevelInfo
	}
}

// New creates a Logger that writes to daily files under dir.
func New(dir string, level Level) *Logger {
	return &Logger{dir: dir, level: level}
}

// Debug logs a debug-level message.
func (l *Logger) Debug(format string, args ...any) {
	l.log(LevelDebug, "DEBUG", format, args...)
}

// Info logs an info-level message.
func (l *Logger) Info(format string, args ...any) {
	l.log(LevelInfo, "INFO", format, args...)
}

// Warn logs a warning-level message.
func (l *Logger) Warn(format string, args ...any) {
	l.log(LevelWarn, "WARN", format, args...)
}

// Error logs an error-level message.
func (l *Logger) Error(format string, args ...any) {
	l.log(LevelError, "ERROR", format, args...)
}

// Writer returns an io.Writer that writes raw lines to the log file.
// Useful for capturing subprocess output.
func (l *Logger) Writer() io.Writer {
	f, err := l.openFile()
	if err != nil {
		return io.Discard
	}
	return f
}

// CurrentLogPath returns the path of the current (today's) log file.
func (l *Logger) CurrentLogPath() string {
	date := time.Now().Format("2006-01-02")
	return filepath.Join(l.dir, date+".log")
}

// Close closes the underlying log file.
func (l *Logger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.file != nil {
		err := l.file.Close()
		l.file = nil
		return err
	}
	return nil
}

// log writes a formatted log line if the message level meets the configured minimum.
func (l *Logger) log(msgLevel Level, label, format string, args ...any) {
	if msgLevel < l.level {
		return
	}
	f, err := l.openFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "logger: failed to open log file: %v\n", err)
		return
	}
	ts := time.Now().Format("2006-01-02 15:04:05")
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(f, "[%s] %s %s\n", ts, label, msg)
}

// openFile returns the current log file, rotating when the date changes.
func (l *Logger) openFile() (*os.File, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	date := time.Now().Format("2006-01-02")
	if l.file != nil && l.date == date {
		return l.file, nil
	}

	// Close previous file if rotating.
	if l.file != nil {
		l.file.Close()
		l.file = nil
	}

	if err := os.MkdirAll(l.dir, 0o750); err != nil {
		return nil, fmt.Errorf("creating log directory %q: %w", l.dir, err)
	}

	path := filepath.Join(l.dir, date+".log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o640)
	if err != nil {
		return nil, fmt.Errorf("opening log file %q: %w", path, err)
	}
	l.file = f
	l.date = date
	return f, nil
}

// CaptureOutput runs cmd, captures combined stdout+stderr, tees it to the logger,
// and returns the combined output string and any error.
func (l *Logger) CaptureOutput(cmd *exec.Cmd) (string, error) {
	logWriter := l.Writer()
	var buf bytes.Buffer
	tee := io.MultiWriter(&buf, logWriter)
	cmd.Stdout = tee
	cmd.Stderr = tee

	err := cmd.Run()
	return buf.String(), err
}
