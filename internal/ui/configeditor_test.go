package ui

import (
	"testing"

	"github.com/teknikqa/upkeep/internal/config"
)

func TestFormatMenuLabel_Bool(t *testing.T) {
	if got := formatMenuLabel("Enabled", true); got != "Enabled [enabled]" {
		t.Errorf("expected 'Enabled [enabled]', got %q", got)
	}
	if got := formatMenuLabel("Enabled", false); got != "Enabled [disabled]" {
		t.Errorf("expected 'Enabled [disabled]', got %q", got)
	}
}

func TestFormatMenuLabel_Int(t *testing.T) {
	if got := formatMenuLabel("Parallelism", 4); got != "Parallelism [4]" {
		t.Errorf("expected 'Parallelism [4]', got %q", got)
	}
}

func TestFormatMenuLabel_String(t *testing.T) {
	if got := formatMenuLabel("Level", "info"); got != "Level [info]" {
		t.Errorf("expected 'Level [info]', got %q", got)
	}
}

func TestFormatMenuLabel_Slice(t *testing.T) {
	got := formatMenuLabel("Skip", []string{"a", "b"})
	if got != "Skip [a, b]" {
		t.Errorf("expected 'Skip [a, b]', got %q", got)
	}
}

func TestFormatMenuLabel_EmptySlice(t *testing.T) {
	got := formatMenuLabel("Skip", []string{})
	if got != "Skip [none]" {
		t.Errorf("expected 'Skip [none]', got %q", got)
	}
}

func TestFormatSlice(t *testing.T) {
	tests := []struct {
		name  string
		items []string
		want  string
	}{
		{"empty", nil, "none"},
		{"one", []string{"a"}, "a"},
		{"three", []string{"a", "b", "c"}, "a, b, c"},
		{"four_truncates", []string{"a", "b", "c", "d"}, "a, b, c (+1 more)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatSlice(tt.items); got != tt.want {
				t.Errorf("formatSlice(%v) = %q, want %q", tt.items, got, tt.want)
			}
		})
	}
}

func TestFormatMap(t *testing.T) {
	got := formatMap(map[string]bool{})
	if got != "none" {
		t.Errorf("expected 'none' for empty map, got %q", got)
	}
}

func TestRemoveFromSlice(t *testing.T) {
	items := []string{"a", "b", "c"}
	result := removeFromSlice(items, "b")
	if len(result) != 2 || result[0] != "a" || result[1] != "c" {
		t.Errorf("expected [a, c], got %v", result)
	}
}

func TestRemoveFromSlice_NotFound(t *testing.T) {
	items := []string{"a", "b"}
	result := removeFromSlice(items, "z")
	if len(result) != 2 {
		t.Errorf("expected unchanged slice, got %v", result)
	}
}

func TestStartsWith(t *testing.T) {
	if !startsWith("Enabled [true]", "Enabled") {
		t.Error("expected true for 'Enabled [true]' starting with 'Enabled'")
	}
	if startsWith("Foo", "FooBar") {
		t.Error("expected false when prefix is longer than string")
	}
}

func TestMapKeys(t *testing.T) {
	m := map[string]bool{"a": true, "b": false}
	keys := mapKeys(m)
	if len(keys) != 2 {
		t.Errorf("expected 2 keys, got %d", len(keys))
	}
}

// --- copyStringSlice ---

func TestCopyStringSlice_Nil(t *testing.T) {
	if got := copyStringSlice(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestCopyStringSlice_Empty(t *testing.T) {
	got := copyStringSlice([]string{})
	if got == nil || len(got) != 0 {
		t.Errorf("expected empty non-nil slice, got %v", got)
	}
}

func TestCopyStringSlice_Independence(t *testing.T) {
	orig := []string{"a", "b", "c"}
	cp := copyStringSlice(orig)
	cp[0] = "MODIFIED"
	if orig[0] != "a" {
		t.Errorf("original slice was mutated: %v", orig)
	}
}

// --- copyStringBoolMap ---

func TestCopyStringBoolMap_Nil(t *testing.T) {
	if got := copyStringBoolMap(nil); got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

func TestCopyStringBoolMap_Independence(t *testing.T) {
	orig := map[string]bool{"x": true, "y": false}
	cp := copyStringBoolMap(orig)
	cp["x"] = false
	if !orig["x"] {
		t.Errorf("original map was mutated")
	}
}

// --- copyConfig ---

func TestCopyConfig_DeepCopy(t *testing.T) {
	cfg := config.Defaults()
	cfg.Providers.Brew.Skip = []string{"formula1", "formula2"}
	cfg.Providers.BrewCask.AuthOverrides = map[string]bool{"app1": true}

	cp := copyConfig(cfg)

	// Mutate the copy.
	cp.Providers.Brew.Skip[0] = "MODIFIED"
	cp.Providers.BrewCask.AuthOverrides["app1"] = false

	// Original must be unchanged.
	if cfg.Providers.Brew.Skip[0] != "formula1" {
		t.Errorf("Brew.Skip[0] was mutated: %q", cfg.Providers.Brew.Skip[0])
	}
	if !cfg.Providers.BrewCask.AuthOverrides["app1"] {
		t.Errorf("BrewCask.AuthOverrides[\"app1\"] was mutated")
	}
}

// --- statusEmoji ---

func TestStatusEmoji_Success(t *testing.T) {
	if got := statusEmoji("success"); got != "✅" {
		t.Errorf("expected ✅, got %q", got)
	}
}

func TestStatusEmoji_Partial(t *testing.T) {
	if got := statusEmoji("partial"); got != "📬" {
		t.Errorf("expected 📬, got %q", got)
	}
}

func TestStatusEmoji_Failed(t *testing.T) {
	if got := statusEmoji("failed"); got != "❌" {
		t.Errorf("expected ❌, got %q", got)
	}
}

func TestStatusEmoji_Skipped(t *testing.T) {
	if got := statusEmoji("skipped"); got != "⏭" {
		t.Errorf("expected ⏭, got %q", got)
	}
}

func TestStatusEmoji_Unknown(t *testing.T) {
	if got := statusEmoji("anything-else"); got != "❓" {
		t.Errorf("expected ❓, got %q", got)
	}
}
