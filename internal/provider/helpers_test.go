package provider_test

import (
	"context"
	"testing"
	"time"

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
