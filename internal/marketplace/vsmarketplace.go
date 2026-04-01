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

// vsMarketplaceFlagsPhase1 is used for the initial batch query.
// 0x2 = IncludeVersions, 0x80 = IncludeLatestVersionOnly, 0x10 = required to return version
// properties (including Microsoft.VisualStudio.Code.PreRelease).
// Note: despite 0x200 being documented as "IncludeVersionProperties", it does NOT cause the API
// to return version properties in practice. 0x10 is the flag that actually works.
const vsMarketplaceFlagsPhase1 = 0x2 | 0x80 | 0x10 // 146: IncludeVersions + IncludeLatestVersionOnly + properties

// vsMarketplaceFlagsPhase2 is used for the follow-up query on extensions where the latest
// version is a pre-release. Omits 0x80 (IncludeLatestVersionOnly) so all versions are returned,
// allowing us to find the latest stable version.
const vsMarketplaceFlagsPhase2 = 0x2 | 0x10 // 18: IncludeVersions + properties (all versions)

// vsMarketplacePreReleaseKey is the version property key that indicates a pre-release extension.
const vsMarketplacePreReleaseKey = "Microsoft.VisualStudio.Code.PreRelease"

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
	Version    string                  `json:"version"`
	Properties []vsMarketplaceProperty `json:"properties"`
}

type vsMarketplaceProperty struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

const vsMarketplaceBatchSize = 100

// GetLatestVersions queries the VS Marketplace for the latest stable version of each extension.
// Extension IDs must be in "publisher.name" format (case-insensitive).
// Uses a two-phase approach: phase 1 fetches only the latest version per extension; if it is a
// pre-release, phase 2 fetches all versions to find the latest stable one.
func (c *VSMarketplaceClient) GetLatestVersions(ctx context.Context, extensionIDs []string) (map[string]LatestVersion, error) {
	if len(extensionIDs) == 0 {
		return map[string]LatestVersion{}, nil
	}

	result := make(map[string]LatestVersion, len(extensionIDs))

	// Phase 1: fetch latest version per extension (with IncludeLatestVersionOnly).
	for start := 0; start < len(extensionIDs); start += vsMarketplaceBatchSize {
		end := start + vsMarketplaceBatchSize
		if end > len(extensionIDs) {
			end = len(extensionIDs)
		}
		batch := extensionIDs[start:end]

		batchResult, err := c.queryBatch(ctx, batch, vsMarketplaceFlagsPhase1)
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

	// Phase 2: for extensions where the latest version is a pre-release, fetch all versions
	// and find the latest stable one.
	var preReleaseIDs []string
	for _, id := range extensionIDs {
		key := strings.ToLower(id)
		lv := result[key]
		if lv.Found && lv.LatestPreReleaseVersion != "" {
			preReleaseIDs = append(preReleaseIDs, key)
		}
	}

	if len(preReleaseIDs) > 0 {
		for start := 0; start < len(preReleaseIDs); start += vsMarketplaceBatchSize {
			end := start + vsMarketplaceBatchSize
			if end > len(preReleaseIDs) {
				end = len(preReleaseIDs)
			}
			batch := preReleaseIDs[start:end]

			allVersionsResult, err := c.queryBatch(ctx, batch, vsMarketplaceFlagsPhase2)
			if err != nil {
				// Non-fatal: keep the pre-release info from phase 1, just mark PreRelease=true.
				for _, id := range batch {
					lv := result[id]
					lv.PreRelease = true
					lv.Version = ""
					result[id] = lv
				}
				continue
			}

			for _, id := range batch {
				phase2 := allVersionsResult[id]
				lv := result[id]
				if phase2.Found && phase2.Version != "" {
					// Found a stable version — update with the stable version.
					lv.Version = phase2.Version
					lv.PreRelease = false
				} else {
					// No stable version found at all — mark as all-pre-release.
					lv.Version = ""
					lv.PreRelease = true
				}
				result[id] = lv
			}
		}
	}

	return result, nil
}

func (c *VSMarketplaceClient) queryBatch(ctx context.Context, extensionIDs []string, flags int) (map[string]LatestVersion, error) {
	criteria := make([]vsMarketplaceCriterion, len(extensionIDs))
	for i, id := range extensionIDs {
		criteria[i] = vsMarketplaceCriterion{FilterType: 7, Value: id}
	}

	reqBody := vsMarketplaceRequest{
		Filters: []vsMarketplaceFilter{{Criteria: criteria}},
		Flags:   flags,
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

// isPreRelease checks if a version's properties include the pre-release marker.
func isPreRelease(props []vsMarketplaceProperty) bool {
	for _, p := range props {
		if p.Key == vsMarketplacePreReleaseKey && p.Value == "true" {
			return true
		}
	}
	return false
}

// findLatestStableVersion iterates versions in order (most recent first) and returns
// the first version that is not marked as a pre-release.
func findLatestStableVersion(versions []vsMarketplaceVersion) (string, bool) {
	for _, v := range versions {
		if !isPreRelease(v.Properties) {
			return v.Version, true
		}
	}
	return "", false
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
			latest := ext.Versions[0]

			if isPreRelease(latest.Properties) {
				// Latest version is a pre-release. Try to find a stable version among all
				// returned versions (covers the phase-2 all-versions response). If none is
				// found, leave Version empty so the phase-2 loop marks PreRelease=true.
				stableVersion, stableFound := findLatestStableVersion(ext.Versions)
				result[id] = LatestVersion{
					ID:                      id,
					Version:                 stableVersion, // "" if no stable version in this response
					Found:                   true,
					LatestPreReleaseVersion: latest.Version,
					PreRelease:              !stableFound,
				}
			} else {
				// Stable version — check if there's also a stable winner among all returned
				// versions (phase 2 path uses this same parser).
				stableVersion, found := findLatestStableVersion(ext.Versions)
				if !found {
					// Shouldn't happen given latest is stable, but be safe.
					stableVersion = latest.Version
				}
				result[id] = LatestVersion{
					ID:      id,
					Version: stableVersion,
					Found:   true,
				}
			}
		}
	}
	return result, nil
}
