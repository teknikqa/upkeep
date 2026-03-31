package ui

import (
	"testing"
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
