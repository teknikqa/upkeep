package provider

import (
	"context"

	"github.com/teknikqa/upkeep/internal/marketplace"
)

// ExportNewRegistry exposes newRegistry for testing purposes.
func ExportNewRegistry() *Registry {
	return newRegistry()
}

// ExportParseBrewOutdated exposes parseBrewOutdated for testing.
// Returns nil items if parsing fails.
func ExportParseBrewOutdated(_ *BrewProvider, jsonStr string) []OutdatedItem {
	items, _ := parseBrewOutdated(jsonStr)
	return items
}

// ParseBrewCaskOutdated exposes parseBrewCaskOutdated for testing.
func ParseBrewCaskOutdated(jsonStr string) ([]OutdatedItem, error) {
	casks, err := parseBrewCaskOutdated(jsonStr)
	if err != nil {
		return nil, err
	}
	items := make([]OutdatedItem, 0, len(casks))
	for _, c := range casks {
		installed := ""
		if len(c.InstalledVersions) > 0 {
			installed = c.InstalledVersions[0]
		}
		items = append(items, OutdatedItem{
			Name:           c.Name,
			CurrentVersion: installed,
			LatestVersion:  c.CurrentVersion,
		})
	}
	return items, nil
}

// DetectAuthRequired exposes detectAuthRequired for testing.
func (p *BrewCaskProvider) DetectAuthRequired(ctx context.Context, name string) bool {
	return p.detectAuthRequired(ctx, name)
}

// BuildDeferredScript builds the deferred cask script content for testing.
func BuildDeferredScript(casks []string) string {
	p := &BrewCaskProvider{}
	return p.buildDeferredScriptContent(casks)
}

// GetByName exposes the global registry's Get function for testing.
func GetByName(name string) (Provider, error) {
	return globalRegistry.Get(name)
}

// ParseNpmOutdated exposes parseNpmOutdated for testing.
func ParseNpmOutdated(jsonStr string) ([]OutdatedItem, error) {
	return parseNpmOutdated(jsonStr)
}

// CommandExistsExport exposes CommandExists for testing.
func CommandExistsExport(name string) bool {
	return CommandExists(name)
}

// ParseComposerDryRun exposes parseComposerDryRun for testing.
func ParseComposerDryRun(output string) []OutdatedItem {
	return parseComposerDryRun(output)
}

// ParsePipOutdated exposes parsePipOutdated for testing.
func ParsePipOutdated(jsonStr string) ([]OutdatedItem, error) {
	return parsePipOutdated(jsonStr)
}

// IsExternallyManaged exposes isExternallyManaged for testing.
func IsExternallyManaged(ctx context.Context) bool {
	return isExternallyManaged(ctx)
}

// SetCheckExternallyManaged sets the PEP 668 detection override on a PipProvider for testing.
func (p *PipProvider) SetCheckExternallyManaged(fn func(ctx context.Context) bool) {
	p.checkExternallyManaged = fn
}

// ParseRustupCheck exposes parseRustupCheck for testing.
func ParseRustupCheck(output string) []OutdatedItem {
	return parseRustupCheck(output)
}

// ParseCargoInstallUpdateList exposes parseCargoInstallUpdateList for testing.
func ParseCargoInstallUpdateList(output string) []OutdatedItem {
	return parseCargoInstallUpdateList(output)
}

// ParseVagrantVersion exposes parseVagrantVersion for testing.
func ParseVagrantVersion(output string) (installed, latest string) {
	return parseVagrantVersion(output)
}

// ParseVirtualBoxVersion strips the build suffix from a VBoxManage --version string.
func ParseVirtualBoxVersion(output string) string {
	p := &VirtualBoxProvider{}
	return p.stripBuildSuffix(output)
}

// ParseExtensionList exposes parseExtensionList for testing.
func ParseExtensionList(output string) []marketplace.Extension {
	return parseExtensionList(output)
}

// CompareVersions exposes compareVersions for testing.
func CompareVersions(installed []marketplace.Extension, latest map[string]marketplace.LatestVersion) []OutdatedItem {
	return compareVersions(installed, latest)
}

// ExportDownloadFile exposes downloadFile for testing.
var ExportDownloadFile = downloadFile
