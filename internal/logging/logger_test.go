package logging_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/teknikqa/upkeep/internal/logging"
)

func TestLogger_CreatesFileWithCorrectDate(t *testing.T) {
	dir := t.TempDir()
	l := logging.New(dir, logging.LevelInfo)
	defer l.Close()

	l.Info("test message")

	date := time.Now().Format("2006-01-02")
	expected := filepath.Join(dir, date+".log")
	if _, err := os.Stat(expected); err != nil {
		t.Errorf("expected log file at %q: %v", expected, err)
	}
}

func TestLogger_AppendMode(t *testing.T) {
	dir := t.TempDir()
	l1 := logging.New(dir, logging.LevelInfo)
	l1.Info("first entry")
	l1.Close()

	// Create a second logger to the same dir — should append.
	l2 := logging.New(dir, logging.LevelInfo)
	l2.Info("second entry")
	l2.Close()

	date := time.Now().Format("2006-01-02")
	path := filepath.Join(dir, date+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "first entry") {
		t.Error("expected first entry to be present")
	}
	if !strings.Contains(content, "second entry") {
		t.Error("expected second entry to be present")
	}
}

func TestLogger_SubprocessCaptureWritesToLog(t *testing.T) {
	dir := t.TempDir()
	l := logging.New(dir, logging.LevelInfo)
	defer l.Close()

	cmd := exec.Command("echo", "hello from subprocess")
	output, err := l.CaptureOutput(cmd)
	if err != nil {
		t.Fatalf("CaptureOutput: %v", err)
	}

	if !strings.Contains(output, "hello from subprocess") {
		t.Errorf("expected output to contain 'hello from subprocess', got %q", output)
	}

	// Also verify the output was written to the log file.
	date := time.Now().Format("2006-01-02")
	path := filepath.Join(dir, date+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !strings.Contains(string(data), "hello from subprocess") {
		t.Errorf("expected log file to contain subprocess output, got %q", string(data))
	}
}

func TestLogger_CurrentLogPath(t *testing.T) {
	dir := t.TempDir()
	l := logging.New(dir, logging.LevelInfo)

	date := time.Now().Format("2006-01-02")
	expected := filepath.Join(dir, date+".log")
	got := l.CurrentLogPath()
	if got != expected {
		t.Errorf("expected log path %q, got %q", expected, got)
	}
}

func TestLogger_LevelFiltering(t *testing.T) {
	dir := t.TempDir()
	// Only log WARN and above.
	l := logging.New(dir, logging.LevelWarn)
	defer l.Close()

	l.Debug("debug message")
	l.Info("info message")
	l.Warn("warn message")

	date := time.Now().Format("2006-01-02")
	path := filepath.Join(dir, date+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "debug message") {
		t.Error("debug message should not appear at WARN level")
	}
	if strings.Contains(content, "info message") {
		t.Error("info message should not appear at WARN level")
	}
	if !strings.Contains(content, "warn message") {
		t.Error("warn message should appear at WARN level")
	}
}

// --- ParseLevel ---

func TestParseLevel_Debug(t *testing.T) {
	if got := logging.ParseLevel("debug"); got != logging.LevelDebug {
		t.Errorf("ParseLevel(\"debug\") = %v, want LevelDebug", got)
	}
}

func TestParseLevel_Info(t *testing.T) {
	if got := logging.ParseLevel("info"); got != logging.LevelInfo {
		t.Errorf("ParseLevel(\"info\") = %v, want LevelInfo", got)
	}
}

func TestParseLevel_Warn(t *testing.T) {
	if got := logging.ParseLevel("warn"); got != logging.LevelWarn {
		t.Errorf("ParseLevel(\"warn\") = %v, want LevelWarn", got)
	}
}

func TestParseLevel_Error(t *testing.T) {
	if got := logging.ParseLevel("error"); got != logging.LevelError {
		t.Errorf("ParseLevel(\"error\") = %v, want LevelError", got)
	}
}

func TestParseLevel_Unknown(t *testing.T) {
	// Unknown strings should default to LevelInfo.
	if got := logging.ParseLevel("unknown-level"); got != logging.LevelInfo {
		t.Errorf("ParseLevel(\"unknown-level\") = %v, want LevelInfo (default)", got)
	}
}

// --- Error ---

func TestLogger_Error_WritesToFile(t *testing.T) {
	dir := t.TempDir()
	l := logging.New(dir, logging.LevelError)
	defer l.Close()

	l.Error("something went wrong: %s", "disk full")

	date := time.Now().Format("2006-01-02")
	path := filepath.Join(dir, date+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "ERROR") {
		t.Errorf("expected 'ERROR' label in log file, got %q", content)
	}
	if !strings.Contains(content, "disk full") {
		t.Errorf("expected error message in log file, got %q", content)
	}
}

// --- Debug ---

func TestLogger_Debug_WritesToFileAtDebugLevel(t *testing.T) {
	dir := t.TempDir()
	l := logging.New(dir, logging.LevelDebug)
	defer l.Close()

	l.Debug("debug detail: %d", 42)

	date := time.Now().Format("2006-01-02")
	path := filepath.Join(dir, date+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "DEBUG") {
		t.Errorf("expected 'DEBUG' label in log file, got %q", content)
	}
	if !strings.Contains(content, "debug detail: 42") {
		t.Errorf("expected debug message in log file, got %q", content)
	}
}

// --- Writer ---

func TestLogger_Writer_ReturnsWritableWriter(t *testing.T) {
	dir := t.TempDir()
	l := logging.New(dir, logging.LevelInfo)
	defer l.Close()

	w := l.Writer()
	if w == nil {
		t.Fatal("expected non-nil Writer()")
	}
	// Write directly and verify it lands in the log file.
	if _, err := w.Write([]byte("raw writer line\n")); err != nil {
		t.Fatalf("writing to Logger.Writer(): %v", err)
	}

	date := time.Now().Format("2006-01-02")
	path := filepath.Join(dir, date+".log")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading log file: %v", err)
	}
	if !strings.Contains(string(data), "raw writer line") {
		t.Errorf("expected log file to contain 'raw writer line', got %q", string(data))
	}
}

// --- Close ---

func TestLogger_Close_Idempotent(t *testing.T) {
	dir := t.TempDir()
	l := logging.New(dir, logging.LevelInfo)
	l.Info("before close")

	if err := l.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	// Second close should be a no-op, not panic or error.
	if err := l.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
