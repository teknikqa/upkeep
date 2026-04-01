// Package marketplace provides clients for querying VS Code extension marketplaces.
package marketplace

import (
	"context"
	"net/http"
	"time"
)

// Extension represents an installed extension with its ID and version.
type Extension struct {
	// ID is the fully-qualified extension identifier: "publisher.name" (lowercased)
	ID      string
	Version string
}

// LatestVersion holds the latest version info for an extension from a marketplace.
type LatestVersion struct {
	ID      string
	Version string
	Found   bool // false if extension was not found in marketplace

	// PreRelease is true when no stable version exists in the marketplace at all
	// (every published version is a pre-release). When true, compareVersions skips
	// the extension entirely — there is nothing stable to upgrade to.
	PreRelease bool

	// LatestPreReleaseVersion is the version string of the most recent pre-release,
	// populated when the marketplace's latest version is a pre-release but a stable
	// version also exists. compareVersions uses this to detect when the user has
	// intentionally installed the pre-release track and should not be flagged as outdated.
	LatestPreReleaseVersion string
}

// Client can query a marketplace for latest extension versions.
type Client interface {
	// GetLatestVersions returns the latest version for each extension ID.
	// Extension IDs are in "publisher.name" format.
	// Returns a map from extension ID → LatestVersion.
	// Per-extension errors result in Found=false; the method only returns a
	// top-level error for fatal failures (e.g. context cancellation).
	GetLatestVersions(ctx context.Context, extensionIDs []string) (map[string]LatestVersion, error)
}

// MarketplaceType identifies which marketplace API to use.
type MarketplaceType string

const (
	VSMarketplace MarketplaceType = "vsmarketplace"
	OpenVSX       MarketplaceType = "openvsx"
)

// EditorMarketplace returns the default marketplace for a given editor command name.
func EditorMarketplace(editor string) MarketplaceType {
	switch editor {
	case "code":
		return VSMarketplace
	default:
		// cursor, kiro, windsurf, agy and anything unknown → Open VSX
		return OpenVSX
	}
}

// DefaultHTTPClient returns an *http.Client with sensible timeouts for marketplace queries.
func DefaultHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 30 * time.Second,
	}
}
