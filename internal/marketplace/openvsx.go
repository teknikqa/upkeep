package marketplace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
)

const openVSXDefaultBaseURL = "https://open-vsx.org"
const openVSXDefaultConcurrency = 5

// openVSXStableSearchLimit is the maximum number of version candidates to check when
// searching for the latest stable version after detecting a pre-release.
const openVSXStableSearchLimit = 20

// OpenVSXClient implements Client for the Open VSX Registry API.
type OpenVSXClient struct {
	httpClient  *http.Client
	baseURL     string
	concurrency int
}

// NewOpenVSXClient creates a client targeting the Open VSX API.
// If httpClient is nil, DefaultHTTPClient() is used.
func NewOpenVSXClient(httpClient *http.Client) *OpenVSXClient {
	return NewOpenVSXClientWithBaseURL(httpClient, openVSXDefaultBaseURL)
}

// NewOpenVSXClientWithBaseURL creates a client with a custom base URL (useful for testing).
// If httpClient is nil, DefaultHTTPClient() is used.
func NewOpenVSXClientWithBaseURL(httpClient *http.Client, baseURL string) *OpenVSXClient {
	if httpClient == nil {
		httpClient = DefaultHTTPClient()
	}
	return &OpenVSXClient{
		httpClient:  httpClient,
		baseURL:     baseURL,
		concurrency: openVSXDefaultConcurrency,
	}
}

// openVSXResponse is the relevant portion of the Open VSX per-extension API response.
type openVSXResponse struct {
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	Version    string `json:"version"`
	PreRelease bool   `json:"preRelease"` // true when this version is a pre-release
	Error      string `json:"error"`      // non-empty on API-level errors
}

// openVSXVersionsResponse is the response from the Open VSX /versions endpoint.
// The API returns versions in descending semantic order (highest first), but the
// "versions" field is a JSON object — Go's map[string]string loses insertion order.
// We use a custom unmarshaler to preserve the API's guaranteed order.
type openVSXVersionsResponse struct {
	Offset    int      `json:"offset"`
	TotalSize int      `json:"totalSize"`
	Versions  []string // version strings in API order (descending semver)
}

// UnmarshalJSON preserves the key insertion order of the "versions" JSON object.
// The Open VSX API guarantees descending semantic version order (sorted by
// semver_major desc, semver_minor desc, semver_patch desc in the database),
// so preserving key order gives us the correct "latest first" sequence.
func (r *openVSXVersionsResponse) UnmarshalJSON(data []byte) error {
	// Decode the outer object manually to handle "versions" specially.
	var raw struct {
		Offset    int             `json:"offset"`
		TotalSize int             `json:"totalSize"`
		Versions  json.RawMessage `json:"versions"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	r.Offset = raw.Offset
	r.TotalSize = raw.TotalSize

	// Parse the "versions" object preserving key order via json.Decoder.
	dec := json.NewDecoder(bytes.NewReader(raw.Versions))
	tok, err := dec.Token() // opening '{'
	if err != nil {
		return fmt.Errorf("parsing versions object: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("expected '{' for versions, got %v", tok)
	}

	for dec.More() {
		keyTok, err := dec.Token() // version string key
		if err != nil {
			return fmt.Errorf("reading version key: %w", err)
		}
		key, ok := keyTok.(string)
		if !ok {
			return fmt.Errorf("expected string key, got %T", keyTok)
		}
		r.Versions = append(r.Versions, key)

		// Skip the URL value — we don't need it.
		var discard string
		if err := dec.Decode(&discard); err != nil {
			return fmt.Errorf("reading version URL for %q: %w", key, err)
		}
	}

	return nil
}

// GetLatestVersions queries Open VSX for the latest stable version of each extension.
// Extension IDs must be in "publisher.name" format (case-insensitive).
// Uses concurrent GET requests bounded by the client's concurrency limit.
func (c *OpenVSXClient) GetLatestVersions(ctx context.Context, extensionIDs []string) (map[string]LatestVersion, error) {
	if len(extensionIDs) == 0 {
		return map[string]LatestVersion{}, nil
	}

	type item struct {
		id  string
		lv  LatestVersion
		err error
	}

	sem := make(chan struct{}, c.concurrency)
	results := make(chan item, len(extensionIDs))

	var wg sync.WaitGroup
	for _, id := range extensionIDs {
		wg.Add(1)
		go func(extID string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			lv, err := c.queryOne(ctx, extID)
			results <- item{id: strings.ToLower(extID), lv: lv, err: err}
		}(id)
	}

	// Close results channel once all goroutines finish.
	go func() {
		wg.Wait()
		close(results)
	}()

	out := make(map[string]LatestVersion, len(extensionIDs))
	var firstErr error
	errCount := 0

	for it := range results {
		if it.err != nil {
			errCount++
			if firstErr == nil {
				firstErr = it.err
			}
			out[it.id] = LatestVersion{ID: it.id, Found: false}
		} else {
			out[it.id] = it.lv
		}
	}

	// Only return a top-level error if ALL requests failed.
	if errCount == len(extensionIDs) {
		return nil, fmt.Errorf("all Open VSX requests failed: %w", firstErr)
	}

	return out, nil
}

func (c *OpenVSXClient) queryOne(ctx context.Context, extensionID string) (LatestVersion, error) {
	key := strings.ToLower(extensionID)

	dotIdx := strings.Index(extensionID, ".")
	if dotIdx <= 0 || dotIdx == len(extensionID)-1 {
		return LatestVersion{ID: key, Found: false}, fmt.Errorf("malformed extension ID %q: expected publisher.name", extensionID)
	}
	namespace := extensionID[:dotIdx]
	name := extensionID[dotIdx+1:]

	url := fmt.Sprintf("%s/api/%s/%s", c.baseURL, namespace, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return LatestVersion{ID: key, Found: false}, fmt.Errorf("creating Open VSX request for %s: %w", extensionID, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return LatestVersion{ID: key, Found: false}, fmt.Errorf("open VSX request for %s failed: %w", extensionID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return LatestVersion{ID: key, Found: false}, nil
	}
	if resp.StatusCode != http.StatusOK {
		return LatestVersion{ID: key, Found: false}, fmt.Errorf("open VSX returned HTTP %d for %s", resp.StatusCode, extensionID)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return LatestVersion{ID: key, Found: false}, fmt.Errorf("reading Open VSX response for %s: %w", extensionID, err)
	}

	var apiResp openVSXResponse
	if err := json.Unmarshal(data, &apiResp); err != nil {
		return LatestVersion{ID: key, Found: false}, fmt.Errorf("parsing Open VSX response for %s: %w", extensionID, err)
	}

	if apiResp.Error != "" {
		return LatestVersion{ID: key, Found: false}, nil
	}
	if apiResp.Version == "" {
		return LatestVersion{ID: key, Found: false}, nil
	}

	// If the latest version is stable, return it directly — no extra requests needed.
	if !apiResp.PreRelease {
		return LatestVersion{
			ID:      key,
			Version: apiResp.Version,
			Found:   true,
		}, nil
	}

	// Latest version is a pre-release. Record it and search for the latest stable version.
	latestPreRelease := apiResp.Version
	stableVersion, found := c.findStableVersion(ctx, namespace, name)
	if !found {
		// All checked versions are pre-release — signal that no stable exists.
		return LatestVersion{
			ID:                      key,
			Version:                 "",
			Found:                   true,
			PreRelease:              true,
			LatestPreReleaseVersion: latestPreRelease,
		}, nil
	}

	return LatestVersion{
		ID:                      key,
		Version:                 stableVersion,
		Found:                   true,
		LatestPreReleaseVersion: latestPreRelease,
	}, nil
}

// findStableVersion fetches the /versions listing for an extension and checks candidates
// (up to openVSXStableSearchLimit) to find the latest stable (non-pre-release) version.
// Returns ("", false) if no stable version is found within the limit.
func (c *OpenVSXClient) findStableVersion(ctx context.Context, namespace, name string) (string, bool) {
	versionsURL := fmt.Sprintf("%s/api/%s/%s/versions", c.baseURL, namespace, name)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, versionsURL, nil)
	if err != nil {
		return "", false
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		if resp != nil {
			resp.Body.Close()
		}
		return "", false
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", false
	}

	var versionsResp openVSXVersionsResponse
	if err := json.Unmarshal(data, &versionsResp); err != nil {
		return "", false
	}

	// versionsResp.Versions is already in API order (descending semver).
	// Check candidates up to the limit.
	candidates := versionsResp.Versions
	limit := openVSXStableSearchLimit
	if limit > len(candidates) {
		limit = len(candidates)
	}

	for _, ver := range candidates[:limit] {
		verURL := fmt.Sprintf("%s/api/%s/%s/%s", c.baseURL, namespace, name, ver)
		verReq, err := http.NewRequestWithContext(ctx, http.MethodGet, verURL, nil)
		if err != nil {
			continue
		}
		verReq.Header.Set("Accept", "application/json")

		verResp, err := c.httpClient.Do(verReq)
		if err != nil || verResp.StatusCode != http.StatusOK {
			if verResp != nil {
				verResp.Body.Close()
			}
			continue
		}

		verData, err := io.ReadAll(verResp.Body)
		verResp.Body.Close()
		if err != nil {
			continue
		}

		var detail openVSXResponse
		if err := json.Unmarshal(verData, &detail); err != nil {
			continue
		}

		if !detail.PreRelease && detail.Version != "" {
			return detail.Version, true
		}
	}

	return "", false
}
