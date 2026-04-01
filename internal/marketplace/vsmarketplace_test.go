package marketplace_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/teknikqa/upkeep/internal/marketplace"
)

const vsMarketplaceSampleResponse = `{
  "results": [{
    "extensions": [
      {
        "extensionName": "vscode-eslint",
        "publisher": {"publisherName": "dbaeumer"},
        "versions": [{"version": "2.4.4", "properties": []}]
      },
      {
        "extensionName": "prettier-vscode",
        "publisher": {"publisherName": "esbenp"},
        "versions": [{"version": "10.2.0", "properties": []}]
      }
    ]
  }]
}`

func TestVSMarketplace_GetLatestVersions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(vsMarketplaceSampleResponse))
	}))
	defer srv.Close()

	client := marketplace.NewVSMarketplaceClientWithBaseURL(srv.Client(), srv.URL)
	ids := []string{"dbaeumer.vscode-eslint", "esbenp.prettier-vscode", "missing.extension"}

	got, err := client.GetLatestVersions(context.Background(), ids)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Found extensions.
	eslint, ok := got["dbaeumer.vscode-eslint"]
	if !ok || !eslint.Found || eslint.Version != "2.4.4" {
		t.Errorf("dbaeumer.vscode-eslint: got %+v, want Found=true Version=2.4.4", eslint)
	}
	prettier, ok := got["esbenp.prettier-vscode"]
	if !ok || !prettier.Found || prettier.Version != "10.2.0" {
		t.Errorf("esbenp.prettier-vscode: got %+v, want Found=true Version=10.2.0", prettier)
	}

	// Missing extension should have Found=false.
	missing, ok := got["missing.extension"]
	if !ok || missing.Found {
		t.Errorf("missing.extension: expected Found=false, got %+v", missing)
	}
}

func TestVSMarketplace_EmptyInput(t *testing.T) {
	client := marketplace.NewVSMarketplaceClientWithBaseURL(nil, "http://should-not-be-called")
	got, err := client.GetLatestVersions(context.Background(), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

func TestVSMarketplace_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := marketplace.NewVSMarketplaceClientWithBaseURL(srv.Client(), srv.URL)
	_, err := client.GetLatestVersions(context.Background(), []string{"pub.ext"})
	if err == nil {
		t.Error("expected error on HTTP 500, got nil")
	}
}

func TestVSMarketplace_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	client := marketplace.NewVSMarketplaceClientWithBaseURL(srv.Client(), srv.URL)
	_, err := client.GetLatestVersions(context.Background(), []string{"pub.ext"})
	if err == nil {
		t.Error("expected error on HTTP 429, got nil")
	}
}

// TestVSMarketplace_SkipsPreRelease verifies the two-phase query: when the phase-1 response
// contains a pre-release latest version, the client fires a phase-2 request and returns the
// latest stable version with LatestPreReleaseVersion populated.
func TestVSMarketplace_SkipsPreRelease(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		// Distinguish phase-1 vs phase-2 by the flags in the request body.
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Flags int `json:"flags"`
		}
		_ = json.Unmarshal(body, &req)

		switch req.Flags {
		case 0x2 | 0x80 | 0x10:
			// Phase 1 (flags=146): latest version is a pre-release.
			_, _ = w.Write([]byte(`{"results":[{"extensions":[{
				"extensionName":"vscode-yaml",
				"publisher":{"publisherName":"redhat"},
				"versions":[{
					"version":"1.22.2026032808",
					"properties":[{"key":"Microsoft.VisualStudio.Code.PreRelease","value":"true"}]
				}]
			}]}]}`))
		case 0x2 | 0x10:
			// Phase 2 (flags=18): all versions — first is pre-release, second is stable.
			_, _ = w.Write([]byte(`{"results":[{"extensions":[{
				"extensionName":"vscode-yaml",
				"publisher":{"publisherName":"redhat"},
				"versions":[
					{"version":"1.22.2026032808","properties":[{"key":"Microsoft.VisualStudio.Code.PreRelease","value":"true"}]},
					{"version":"1.21.0","properties":[]}
				]
			}]}]}`))
		default:
			t.Errorf("unexpected flags: %d", req.Flags)
			_, _ = w.Write([]byte(`{"results":[]}`))
		}
	}))
	defer srv.Close()

	client := marketplace.NewVSMarketplaceClientWithBaseURL(srv.Client(), srv.URL)
	got, err := client.GetLatestVersions(context.Background(), []string{"redhat.vscode-yaml"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestCount != 2 {
		t.Errorf("expected 2 requests (phase1 + phase2), got %d", requestCount)
	}
	lv, ok := got["redhat.vscode-yaml"]
	if !ok || !lv.Found {
		t.Fatalf("redhat.vscode-yaml: expected Found=true, got %+v", lv)
	}
	if lv.Version != "1.21.0" {
		t.Errorf("expected stable version 1.21.0, got %q", lv.Version)
	}
	if lv.LatestPreReleaseVersion != "1.22.2026032808" {
		t.Errorf("expected LatestPreReleaseVersion=1.22.2026032808, got %q", lv.LatestPreReleaseVersion)
	}
	if lv.PreRelease {
		t.Errorf("expected PreRelease=false (stable exists), got true")
	}
}

// TestVSMarketplace_AllPreRelease verifies that when all versions are pre-releases,
// the client marks PreRelease=true with no stable version.
func TestVSMarketplace_AllPreRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Both phase 1 and phase 2 return only pre-release versions.
		_, _ = w.Write([]byte(`{"results":[{"extensions":[{
			"extensionName":"myext",
			"publisher":{"publisherName":"mypub"},
			"versions":[
				{"version":"2.0.0-pre","properties":[{"key":"Microsoft.VisualStudio.Code.PreRelease","value":"true"}]},
				{"version":"1.0.0-pre","properties":[{"key":"Microsoft.VisualStudio.Code.PreRelease","value":"true"}]}
			]
		}]}]}`))
	}))
	defer srv.Close()

	client := marketplace.NewVSMarketplaceClientWithBaseURL(srv.Client(), srv.URL)
	got, err := client.GetLatestVersions(context.Background(), []string{"mypub.myext"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lv, ok := got["mypub.myext"]
	if !ok || !lv.Found {
		t.Fatalf("mypub.myext: expected Found=true, got %+v", lv)
	}
	if !lv.PreRelease {
		t.Errorf("expected PreRelease=true (all versions are pre-release), got false")
	}
	if lv.Version != "" {
		t.Errorf("expected empty stable Version when all are pre-release, got %q", lv.Version)
	}
}

// TestVSMarketplace_StableLatest verifies that when the latest version is already stable,
// no second request is made.
func TestVSMarketplace_StableLatest(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"results":[{"extensions":[{
			"extensionName":"vscode-eslint",
			"publisher":{"publisherName":"dbaeumer"},
			"versions":[{"version":"2.4.4","properties":[]}]
		}]}]}`))
	}))
	defer srv.Close()

	client := marketplace.NewVSMarketplaceClientWithBaseURL(srv.Client(), srv.URL)
	got, err := client.GetLatestVersions(context.Background(), []string{"dbaeumer.vscode-eslint"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("expected exactly 1 request for stable extension, got %d", requestCount)
	}
	lv, ok := got["dbaeumer.vscode-eslint"]
	if !ok || !lv.Found {
		t.Fatalf("dbaeumer.vscode-eslint: expected Found=true, got %+v", lv)
	}
	if lv.Version != "2.4.4" {
		t.Errorf("expected version 2.4.4, got %q", lv.Version)
	}
	if lv.PreRelease {
		t.Errorf("expected PreRelease=false for stable extension")
	}
	if lv.LatestPreReleaseVersion != "" {
		t.Errorf("expected empty LatestPreReleaseVersion for stable extension, got %q", lv.LatestPreReleaseVersion)
	}
}

func TestVSMarketplace_CaseInsensitive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Return extension with mixed case publisher name.
		_, _ = w.Write([]byte(`{"results":[{"extensions":[{
			"extensionName":"MyExt",
			"publisher":{"publisherName":"MyPublisher"},
			"versions":[{"version":"1.0.0"}]
		}]}]}`))
	}))
	defer srv.Close()

	client := marketplace.NewVSMarketplaceClientWithBaseURL(srv.Client(), srv.URL)
	got, err := client.GetLatestVersions(context.Background(), []string{"mypublisher.myext"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lv, ok := got["mypublisher.myext"]
	if !ok || !lv.Found {
		t.Errorf("expected mypublisher.myext to be found, got %+v", lv)
	}
}
