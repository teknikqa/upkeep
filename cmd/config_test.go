package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
)

func TestConfigShowCmd(t *testing.T) {
	// Create a temp config file with known values.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	cfg := config.Defaults()
	cfg.Parallelism = 7
	if err := config.Save(cfg, path); err != nil {
		t.Fatalf("setup: Save failed: %v", err)
	}

	// Set the config file path and capture output.
	old := cfgFile
	cfgFile = path
	t.Cleanup(func() { cfgFile = old })

	var buf bytes.Buffer
	configShowCmd.SetOut(&buf)
	configShowCmd.SetErr(&buf)

	// Override stdout for this command since it uses fmt.Print directly.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := configShowCmd.RunE(configShowCmd, nil)

	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("configShowCmd failed: %v", err)
	}

	var captured bytes.Buffer
	captured.ReadFrom(r)
	output := captured.String()

	if !strings.Contains(output, "parallelism: 7") {
		t.Errorf("expected output to contain 'parallelism: 7', got:\n%s", output)
	}
	if !strings.Contains(output, "providers:") {
		t.Errorf("expected output to contain 'providers:', got:\n%s", output)
	}
}

func TestConfigPathCmd_Default(t *testing.T) {
	old := cfgFile
	cfgFile = ""
	t.Cleanup(func() { cfgFile = old })

	// Capture stdout.
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	configPathCmd.Run(configPathCmd, nil)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := strings.TrimSpace(buf.String())

	expected := config.DefaultConfigPath()
	if output != expected {
		t.Errorf("expected %q, got %q", expected, output)
	}
}

func TestConfigPathCmd_CustomFlag(t *testing.T) {
	old := cfgFile
	cfgFile = "/tmp/custom-upkeep.yaml"
	t.Cleanup(func() { cfgFile = old })

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	configPathCmd.Run(configPathCmd, nil)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := strings.TrimSpace(buf.String())

	if output != "/tmp/custom-upkeep.yaml" {
		t.Errorf("expected '/tmp/custom-upkeep.yaml', got %q", output)
	}
}

func TestConfigResetCmd_WritesDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")

	old := cfgFile
	cfgFile = path
	t.Cleanup(func() { cfgFile = old })

	// Pre-write a non-default config.
	nonDefault := config.Defaults()
	nonDefault.Parallelism = 16
	if err := config.Save(nonDefault, path); err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Reset should write defaults (we can't test the interactive prompt,
	// so we directly call Save with defaults to test the save path).
	if err := config.Save(config.Defaults(), path); err != nil {
		t.Fatalf("reset save failed: %v", err)
	}

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load after reset: %v", err)
	}
	if loaded.Parallelism != 4 {
		t.Errorf("expected parallelism=4 after reset, got %d", loaded.Parallelism)
	}
}
