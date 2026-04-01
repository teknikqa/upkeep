package marketplace

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const vsMarketplaceDefaultBaseURL = "https://marketplace.visualstudio.com"

// vsMarketplaceQueryFlags controls what data is returned by the VS Marketplace API.
// 0x2 (IncludeVersions) | 0x10 (IncludeFiles) | 0x80 (IncludeLatestVersionOnly) |
// 0x100 (Unpublished — excluded) | 0x200 (IncludeVersionProperties)
// Using 0x2 | 0x80 = 130 gives us versions with latest-only flag.
const vsMarketplaceFlags = 0x2 | 0x80 // 130: IncludeVersions + IncludeLatestVersionOnly

// VSMarketplaceClient implements Client for the Visual Studio Marketplace API.
type VSMarketplaceClient struct {
	httpClient *http.Client
	baseURL    string
}

// NewVSMarketplaceClient creates a client targeting the VS Marketplace API.
// If httpClient is nil, DefaultHTTPClient() is used.
func NewVSMarketplaceClient(httpClient *http.Client) *VSMarketplaceClient {
	return NewVSMarketplaceClientWithBaseURL(httpClient, vsMarketplaceDefaultBaseURL)
}

// NewVSMarketplaceClientWithBaseURL creates a client with a custom base URL (useful for testing).
// If httpClient is nil, DefaultHTTPClient() is used.
func NewVSMarketplaceClientWithBaseURL(httpClient *http.Client, baseURL string) *VSMarketplaceClient {
	if httpClient == nil {
		httpClient = DefaultHTTPClient()
	}
	return &VSMarketplaceClient{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

// vsMarketplaceRequest is the JSON request body for the extension query API.
type vsMarketplaceRequest struct {
	Filters []vsMarketplaceFilter `json:"filters"`
	Flags   int                   `json:"flags"`
}

type vsMarketplaceFilter struct {
	Criteria []vsMarketplaceCriterion `json:"criteria"`
}

type vsMarketplaceCriterion struct {
	FilterType int    `json:"filterType"`
	Value      string `json:"value"`
}

// vsMarketplaceResponse is the JSON response from the extension query API.
type vsMarketplaceResponse struct {
	Results []vsMarketplaceResult `json:"results"`
}

type vsMarketplaceResult struct {
	Extensions []vsMarketplaceExtension `json:"extensions"`
}

type vsMarketplaceExtension struct {
	ExtensionName string                 `json:"extensionName"`
	Publisher     vsMarketplacePublisher `json:"publisher"`
	Versions      []vsMarketplaceVersion `json:"versions"`
}

type vsMarketplacePublisher struct {
	PublisherName string `json:"publisherName"`
}

type vsMarketplaceVersion struct {
	Version string `json:"version"`
}

const vsMarketplaceBatchSize = 100

// GetLatestVersions queries the VS Marketplace for the latest version of each extension.
// Extension IDs must be in "publisher.name" format (case-insensitive).
func (c *VSMarketplaceClient) GetLatestVersions(ctx context.Context, extensionIDs []string) (map[string]LatestVersion, error) {
	if len(extensionIDs) == 0 {
		return map[string]LatestVersion{}, nil
	}

	result := make(map[string]LatestVersion, len(extensionIDs))

	// Process in batches of vsMarketplaceBatchSize.
	for start := 0; start < len(extensionIDs); start += vsMarketplaceBatchSize {
		end := start + vsMarketplaceBatchSize
		if end > len(extensionIDs) {
			end = len(extensionIDs)
		}
		batch := extensionIDs[start:end]

		batchResult, err := c.queryBatch(ctx, batch)
		if err != nil {
			return nil, err
		}
		for k, v := range batchResult {
			result[k] = v
		}
	}

	// Fill in Found=false for any IDs not returned by the API.
	for _, id := range extensionIDs {
		key := strings.ToLower(id)
		if _, ok := result[key]; !ok {
			result[key] = LatestVersion{ID: key, Found: false}
		}
	}

	return result, nil
}

func (c *VSMarketplaceClient) queryBatch(ctx context.Context, extensionIDs []string) (map[string]LatestVersion, error) {
	criteria := make([]vsMarketplaceCriterion, len(extensionIDs))
	for i, id := range extensionIDs {
		criteria[i] = vsMarketplaceCriterion{FilterType: 7, Value: id}
	}

	reqBody := vsMarketplaceRequest{
		Filters: []vsMarketplaceFilter{{Criteria: criteria}},
		Flags:   vsMarketplaceFlags,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshalling VS Marketplace request: %w", err)
	}

	url := c.baseURL + "/_apis/public/gallery/extensionquery"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating VS Marketplace request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json;api-version=6.1-preview.1")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("VS Marketplace request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("VS Marketplace rate limited (HTTP 429)")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("VS Marketplace returned HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading VS Marketplace response: %w", err)
	}

	return parseVSMarketplaceResponse(data)
}

func parseVSMarketplaceResponse(data []byte) (map[string]LatestVersion, error) {
	var resp vsMarketplaceResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing VS Marketplace response: %w", err)
	}

	result := make(map[string]LatestVersion)
	for _, r := range resp.Results {
		for _, ext := range r.Extensions {
			if len(ext.Versions) == 0 {
				continue
			}
			id := strings.ToLower(ext.Publisher.PublisherName + "." + ext.ExtensionName)
			result[id] = LatestVersion{
				ID:      id,
				Version: ext.Versions[0].Version,
				Found:   true,
			}
		}
	}
	return result, nil
}
