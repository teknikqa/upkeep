package marketplace_test

import (
	"context"
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
        "versions": [{"version": "2.4.4"}]
      },
      {
        "extensionName": "prettier-vscode",
        "publisher": {"publisherName": "esbenp"},
        "versions": [{"version": "10.2.0"}]
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
