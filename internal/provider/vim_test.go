package provider_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
	"github.com/teknikqa/upkeep/internal/provider"
)

func TestVimProvider_Name(t *testing.T) {
	p := provider.NewVimProvider(config.VimConfig{Enabled: true}, nil)
	if p.Name() != "vim" {
		t.Errorf("expected %q, got %q", "vim", p.Name())
	}
	if p.DisplayName() != "Vim Plugins" {
		t.Errorf("expected %q, got %q", "Vim Plugins", p.DisplayName())
	}
}

func TestVimProvider_DependsOn(t *testing.T) {
	p := provider.NewVimProvider(config.VimConfig{Enabled: true}, nil)
	if deps := p.DependsOn(); len(deps) != 0 {
		t.Errorf("expected no dependencies, got %v", deps)
	}
}

func TestVimProvider_Scan_NoPathogen_NoBundles(t *testing.T) {
	p := provider.NewVimProvider(config.VimConfig{
		Enabled:     true,
		PathogenDir: "/nonexistent/vim/autoload",
		BundlesDir:  "/nonexistent/vim/bundle",
	}, nil)
	result := p.Scan(context.Background())
	if result.Available {
		t.Error("expected Available=false when no pathogen or bundles")
	}
}

func TestVimProvider_Scan_WithPathogen(t *testing.T) {
	dir := t.TempDir()
	p := provider.NewVimProvider(config.VimConfig{
		Enabled:     true,
		PathogenDir: dir,
		BundlesDir:  "/nonexistent/vim/bundle",
	}, nil)
	result := p.Scan(context.Background())
	if !result.Available {
		t.Error("expected Available=true when pathogen dir exists")
	}
	if len(result.Outdated) == 0 {
		t.Error("expected at least one outdated item (pathogen)")
	}
}

func TestVimProvider_Scan_WithBundles(t *testing.T) {
	dir := t.TempDir()
	// Create a fake git bundle.
	bundlePath := filepath.Join(dir, "nerdtree")
	os.MkdirAll(filepath.Join(bundlePath, ".git"), 0o755)

	p := provider.NewVimProvider(config.VimConfig{
		Enabled:    true,
		BundlesDir: dir,
	}, nil)
	result := p.Scan(context.Background())
	if !result.Available {
		t.Error("expected Available=true when bundles exist")
	}
}

func TestVimProvider_Update_ExternalScript(t *testing.T) {
	// Create a simple executable script.
	dir := t.TempDir()
	scriptPath := filepath.Join(dir, "update-vim.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("creating test script: %v", err)
	}

	p := provider.NewVimProvider(config.VimConfig{
		Enabled:      true,
		UpdateScript: scriptPath,
	}, nil)

	items := []provider.OutdatedItem{{Name: "vim (external script)"}}
	result := p.Update(context.Background(), items)
	if len(result.Updated) != 1 || result.Updated[0] != "vim" {
		t.Errorf("expected vim in updated, got %+v", result)
	}
}

func TestVimProvider_Registered(t *testing.T) {
	p, err := provider.GetByName("vim")
	if err != nil {
		t.Fatalf("vim not registered: %v", err)
	}
	if p.Name() != "vim" {
		t.Errorf("expected vim, got %s", p.Name())
	}
}

// TestDownloadFile_Success verifies that a 200 response body is written to dest.
func TestDownloadFile_Success(t *testing.T) {
	const content = "pathogen content"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(content))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "pathogen.vim")
	err := provider.ExportDownloadFile(context.Background(), srv.URL, dest)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("reading dest file: %v", err)
	}
	if string(got) != content {
		t.Errorf("expected %q, got %q", content, string(got))
	}
}

// TestDownloadFile_Non200 verifies that a non-200 HTTP status returns an error.
func TestDownloadFile_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "pathogen.vim")
	err := provider.ExportDownloadFile(context.Background(), srv.URL, dest)
	if err == nil {
		t.Fatal("expected error for 404 response, got nil")
	}
}

// TestDownloadFile_ContextCancelled verifies that a cancelled context returns an error.
func TestDownloadFile_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel

	dest := filepath.Join(t.TempDir(), "pathogen.vim")
	// Any URL will do — context is already cancelled before the request starts.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	err := provider.ExportDownloadFile(ctx, srv.URL, dest)
	if err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}

// TestDownloadFile_CreatesParentDirs verifies that missing parent directories are
// created automatically before writing the destination file.
func TestDownloadFile_CreatesParentDirs(t *testing.T) {
	const content = "data"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(content))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "deep", "nested", "dir", "pathogen.vim")
	err := provider.ExportDownloadFile(context.Background(), srv.URL, dest)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if _, err := os.Stat(dest); err != nil {
		t.Errorf("expected dest file to exist: %v", err)
	}
}
