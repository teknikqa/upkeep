package marketplace_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
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

// TestOpenVSX_SkipsPreRelease verifies that when the main endpoint returns preRelease=true,
// the client fetches /versions and then checks individual version details to find the first
// stable version, returning it with LatestPreReleaseVersion populated.
func TestOpenVSX_SkipsPreRelease(t *testing.T) {
	requestPaths := []string{}
	var mu sync.Mutex

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestPaths = append(requestPaths, r.URL.Path)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/api/redhat/vscode-yaml":
			// Main endpoint returns a pre-release.
			_, _ = w.Write([]byte(`{"namespace":"redhat","name":"vscode-yaml","version":"1.22.2026032808","preRelease":true}`))
		case "/api/redhat/vscode-yaml/versions":
			// Versions map: two entries.
			_, _ = w.Write([]byte(`{"offset":0,"totalSize":2,"versions":{"1.22.2026032808":"https://example.com","1.21.0":"https://example.com"}}`))
		case "/api/redhat/vscode-yaml/1.22.2026032808":
			_, _ = w.Write([]byte(`{"namespace":"redhat","name":"vscode-yaml","version":"1.22.2026032808","preRelease":true}`))
		case "/api/redhat/vscode-yaml/1.21.0":
			_, _ = w.Write([]byte(`{"namespace":"redhat","name":"vscode-yaml","version":"1.21.0","preRelease":false}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := marketplace.NewOpenVSXClientWithBaseURL(srv.Client(), srv.URL)
	got, err := client.GetLatestVersions(context.Background(), []string{"redhat.vscode-yaml"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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

// TestOpenVSX_AllPreRelease verifies that when all version candidates are pre-releases,
// the client returns PreRelease=true and an empty stable Version.
func TestOpenVSX_AllPreRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/mypub/myext":
			_, _ = w.Write([]byte(`{"namespace":"mypub","name":"myext","version":"2.0.0-pre","preRelease":true}`))
		case "/api/mypub/myext/versions":
			_, _ = w.Write([]byte(`{"offset":0,"totalSize":2,"versions":{"2.0.0-pre":"https://example.com","1.0.0-pre":"https://example.com"}}`))
		case "/api/mypub/myext/2.0.0-pre":
			_, _ = w.Write([]byte(`{"namespace":"mypub","name":"myext","version":"2.0.0-pre","preRelease":true}`))
		case "/api/mypub/myext/1.0.0-pre":
			_, _ = w.Write([]byte(`{"namespace":"mypub","name":"myext","version":"1.0.0-pre","preRelease":true}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := marketplace.NewOpenVSXClientWithBaseURL(srv.Client(), srv.URL)
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

// TestOpenVSX_StableLatest verifies that when the main endpoint returns preRelease=false,
// no additional requests are made for the /versions endpoint.
func TestOpenVSX_StableLatest(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"namespace":"dbaeumer","name":"vscode-eslint","version":"2.4.4","preRelease":false}`))
	}))
	defer srv.Close()

	client := marketplace.NewOpenVSXClientWithBaseURL(srv.Client(), srv.URL)
	got, err := client.GetLatestVersions(context.Background(), []string{"dbaeumer.vscode-eslint"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if requestCount != 1 {
		t.Errorf("expected exactly 1 request for stable extension, got %d", requestCount)
	}
	lv := got["dbaeumer.vscode-eslint"]
	if !lv.Found || lv.Version != "2.4.4" {
		t.Errorf("expected Found=true Version=2.4.4, got %+v", lv)
	}
	if lv.PreRelease {
		t.Errorf("expected PreRelease=false for stable extension")
	}
	if lv.LatestPreReleaseVersion != "" {
		t.Errorf("expected empty LatestPreReleaseVersion, got %q", lv.LatestPreReleaseVersion)
	}
}

// TestOpenVSX_PreReleaseFieldMissing verifies that when preRelease is absent from the
// response (defaults to false), the extension is treated as stable.
func TestOpenVSX_PreReleaseFieldMissing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// No "preRelease" field — should default to false (stable).
		_, _ = w.Write([]byte(`{"namespace":"pub","name":"ext","version":"1.5.0"}`))
	}))
	defer srv.Close()

	client := marketplace.NewOpenVSXClientWithBaseURL(srv.Client(), srv.URL)
	got, err := client.GetLatestVersions(context.Background(), []string{"pub.ext"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lv := got["pub.ext"]
	if !lv.Found || lv.Version != "1.5.0" {
		t.Errorf("expected Found=true Version=1.5.0, got %+v", lv)
	}
	if lv.PreRelease {
		t.Errorf("expected PreRelease=false when field is missing")
	}
}

// TestOpenVSX_VersionOrderPreserved verifies that the client respects the API's semantic
// version ordering rather than using lexicographic sort. Lexicographic sort would put "1.9.0"
// before "1.10.0" — this test has "1.9.0" as pre-release and "1.10.0" as the correct stable
// version, which would be missed if the client sorted lexicographically and picked "1.9.0".
func TestOpenVSX_VersionOrderPreserved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/pub/ext":
			_, _ = w.Write([]byte(`{"namespace":"pub","name":"ext","version":"1.11.2026040100","preRelease":true}`))
		case "/api/pub/ext/versions":
			// API returns in descending semver order: 1.11.x > 1.10.0 > 1.9.0.
			// Lexicographic sort would wrongly order: 1.9.0 > 1.11.x > 1.10.0.
			_, _ = w.Write([]byte(`{"offset":0,"totalSize":3,"versions":{"1.11.2026040100":"https://example.com","1.10.0":"https://example.com","1.9.0":"https://example.com"}}`))
		case "/api/pub/ext/1.11.2026040100":
			_, _ = w.Write([]byte(`{"namespace":"pub","name":"ext","version":"1.11.2026040100","preRelease":true}`))
		case "/api/pub/ext/1.10.0":
			// First stable version in API order — should be picked.
			_, _ = w.Write([]byte(`{"namespace":"pub","name":"ext","version":"1.10.0","preRelease":false}`))
		case "/api/pub/ext/1.9.0":
			// Older stable — should NOT be picked (1.10.0 is newer).
			_, _ = w.Write([]byte(`{"namespace":"pub","name":"ext","version":"1.9.0","preRelease":false}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	client := marketplace.NewOpenVSXClientWithBaseURL(srv.Client(), srv.URL)
	got, err := client.GetLatestVersions(context.Background(), []string{"pub.ext"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lv := got["pub.ext"]
	if !lv.Found {
		t.Fatalf("expected Found=true, got %+v", lv)
	}
	if lv.Version != "1.10.0" {
		t.Errorf("expected stable version 1.10.0 (API order), got %q (lexicographic sort would give 1.9.0)", lv.Version)
	}
}
