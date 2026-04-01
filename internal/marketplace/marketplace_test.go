package marketplace_test

import (
	"testing"

	"github.com/teknikqa/upkeep/internal/marketplace"
)

func TestEditorMarketplace(t *testing.T) {
	tests := []struct {
		editor string
		want   marketplace.MarketplaceType
	}{
		{"code", marketplace.VSMarketplace},
		{"cursor", marketplace.OpenVSX},
		{"kiro", marketplace.OpenVSX},
		{"windsurf", marketplace.OpenVSX},
		{"agy", marketplace.OpenVSX},
		{"unknown-editor", marketplace.OpenVSX},
	}
	for _, tt := range tests {
		got := marketplace.EditorMarketplace(tt.editor)
		if got != tt.want {
			t.Errorf("EditorMarketplace(%q) = %q, want %q", tt.editor, got, tt.want)
		}
	}
}
