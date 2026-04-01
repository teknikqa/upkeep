package marketplace_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/teknikqa/upkeep/internal/marketplace"
)

func TestOpenVSX_GetLatestVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Route: GET /api/{namespace}/{extension}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/"), "/")
		if len(parts) != 2 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		ns, name := parts[0], parts[1]
		switch strings.ToLower(ns + "." + name) {
		case "dbaeumer.vscode-eslint":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"namespace":"dbaeumer","name":"vscode-eslint","version":"2.4.4"}`))
		case "esbenp.prettier-vscode":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"namespace":"esbenp","name":"prettier-vscode","version":"10.2.0"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := marketplace.NewOpenVSXClientWithBaseURL(srv.Client(), srv.URL)
	ids := []string{"dbaeumer.vscode-eslint", "esbenp.prettier-vscode", "missing.extension"}

	got, err := client.GetLatestVersions(context.Background(), ids)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	eslint := got["dbaeumer.vscode-eslint"]
	if !eslint.Found || eslint.Version != "2.4.4" {
		t.Errorf("dbaeumer.vscode-eslint: got %+v, want Found=true Version=2.4.4", eslint)
	}

	prettier := got["esbenp.prettier-vscode"]
	if !prettier.Found || prettier.Version != "10.2.0" {
		t.Errorf("esbenp.prettier-vscode: got %+v, want Found=true Version=10.2.0", prettier)
	}

	missing := got["missing.extension"]
	if missing.Found {
		t.Errorf("missing.extension: expected Found=false, got %+v", missing)
	}
}

func TestOpenVSX_EmptyInput(t *testing.T) {
	client := marketplace.NewOpenVSXClientWithBaseURL(nil, "http://should-not-be-called")
	got, err := client.GetLatestVersions(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestOpenVSX_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := marketplace.NewOpenVSXClientWithBaseURL(srv.Client(), srv.URL)
	got, err := client.GetLatestVersions(context.Background(), []string{"pub.ext"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lv := got["pub.ext"]
	if lv.Found {
		t.Errorf("expected Found=false for 404, got %+v", lv)
	}
}

func TestOpenVSX_MalformedExtensionID(t *testing.T) {
	// Extension ID with no dot should not crash; returns Found=false.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := marketplace.NewOpenVSXClientWithBaseURL(srv.Client(), srv.URL)
	got, err := client.GetLatestVersions(context.Background(), []string{"nodot"})
	// Malformed IDs result in Found=false; not a fatal error (partial failure).
	if err != nil {
		// Could be all-failed error; that's acceptable.
		t.Logf("got error (acceptable for malformed ID): %v", err)
		return
	}
	lv, ok := got["nodot"]
	if ok && lv.Found {
		t.Errorf("expected Found=false for malformed ID, got %+v", lv)
	}
}

func TestOpenVSX_ExtensionIDSplit(t *testing.T) {
	// Verify the namespace/name split is correct for a compound extension name.
	// e.g. "pub.my-ext" → namespace=pub, name=my-ext
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/pub/my-ext" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"namespace":"pub","name":"my-ext","version":"3.0.0"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	client := marketplace.NewOpenVSXClientWithBaseURL(srv.Client(), srv.URL)
	got, err := client.GetLatestVersions(context.Background(), []string{"pub.my-ext"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lv := got["pub.my-ext"]
	if !lv.Found || lv.Version != "3.0.0" {
		t.Errorf("expected Found=true Version=3.0.0, got %+v", lv)
	}
}
