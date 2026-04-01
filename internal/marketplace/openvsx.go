package marketplace

import (
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
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Version   string `json:"version"`
	Error     string `json:"error"` // non-empty on API-level errors
}

// GetLatestVersions queries Open VSX for the latest version of each extension.
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

	return LatestVersion{
		ID:      key,
		Version: apiResp.Version,
		Found:   true,
	}, nil
}
