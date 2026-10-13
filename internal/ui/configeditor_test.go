package ui

import (
	"errors"
	"reflect"
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
	cfg.Providers.Mas.Skip = []string{"Xcode"}
	cfg.Providers.Macports.Skip = []string{"python312"}

	cp := copyConfig(cfg)

	// Mutate the copy.
	cp.Providers.Brew.Skip[0] = "MODIFIED"
	cp.Providers.Mas.Skip[0] = "MODIFIED"
	cp.Providers.Macports.Skip[0] = "MODIFIED"
	cp.Providers.BrewCask.AuthOverrides["app1"] = false

	// Original must be unchanged.
	if cfg.Providers.Brew.Skip[0] != "formula1" {
		t.Errorf("Brew.Skip[0] was mutated: %q", cfg.Providers.Brew.Skip[0])
	}
	if !cfg.Providers.BrewCask.AuthOverrides["app1"] {
		t.Errorf("BrewCask.AuthOverrides[\"app1\"] was mutated")
	}
	if cfg.Providers.Mas.Skip[0] != "Xcode" {
		t.Errorf("Mas.Skip[0] was mutated: %q", cfg.Providers.Mas.Skip[0])
	}
	if cfg.Providers.Macports.Skip[0] != "python312" {
		t.Errorf("Macports.Skip[0] was mutated: %q", cfg.Providers.Macports.Skip[0])
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

// --- Menu navigation (scripted prompts) ---

// scriptPrompts replaces the prompt seams for one test. Each select answer is
// matched as a prefix of the offered options; confirms are returned in order.
func scriptPrompts(t *testing.T, selects []string, confirms ...bool) {
	t.Helper()
	origSelect, origConfirm := selectOption, confirmBool
	t.Cleanup(func() {
		selectOption, confirmBool = origSelect, origConfirm
		if len(selects) > 0 || len(confirms) > 0 {
			t.Errorf("unused script: selects=%v confirms=%v", selects, confirms)
		}
	})
	selectOption = func(label string, options []string) (string, error) {
		if len(selects) == 0 {
			t.Fatalf("unexpected select %q", label)
		}
		want := selects[0]
		selects = selects[1:]
		for _, o := range options {
			if startsWith(o, want) {
				return o, nil
			}
		}
		t.Fatalf("%q: no option starting with %q in %v", label, want, options)
		return "", nil
	}
	confirmBool = func(label string, _ bool) (bool, error) {
		if len(confirms) == 0 {
			t.Fatalf("unexpected confirm %q", label)
		}
		v := confirms[0]
		confirms = confirms[1:]
		return v, nil
	}
}

func TestEditProvidersSection_OpensEveryProvider(t *testing.T) {
	providers := []string{
		"Brew Cask", "Brew", "npm", "pnpm", "Yarn", "Bun", "Composer", "pip",
		"Rust", "uv", "Editor Extensions", "Oh My Zsh", "Vim", "Vagrant",
		"Mac App Store", "MacPorts",
	}
	var script []string
	for _, p := range providers {
		script = append(script, p, "Back")
	}
	scriptPrompts(t, append(script, "Back"))

	if err := editProvidersSection(config.Defaults()); err != nil {
		t.Fatal(err)
	}
}

func TestEditProvidersSection_EditsNewProviders(t *testing.T) {
	cfg := config.Defaults()
	scriptPrompts(t, []string{
		"pnpm", "Enabled", "Back",
		"Yarn", "Enabled", "Back",
		"Bun", "Enabled", "Back",
		"uv", "Self Update", "Tools", "Enabled", "Back",
		"Mac App Store", "Enabled", "Skip", "Back", "Back",
		"MacPorts", "Enabled", "Back",
		"Editor Extensions", "Enabled", "Back",
		"Back",
	}, false, false, false, false, false, false, false, false, false)

	if err := editProvidersSection(cfg); err != nil {
		t.Fatal(err)
	}
	p := cfg.Providers
	for name, enabled := range map[string]bool{
		"pnpm": p.Pnpm.Enabled, "yarn": p.Yarn.Enabled, "bun": p.Bun.Enabled,
		"uv": p.Uv.Enabled, "uv self_update": p.Uv.SelfUpdate, "uv tool": p.Uv.Tool,
		"mas": p.Mas.Enabled, "macports": p.Macports.Enabled,
		"editor": p.Editor.Enabled,
	} {
		if enabled {
			t.Errorf("%s still enabled", name)
		}
	}
}

func TestEditProvidersSection_PropagatesPromptErrors(t *testing.T) {
	scriptPrompts(t, []string{"Bun", "Enabled"})
	confirmBool = func(string, bool) (bool, error) { return false, errors.New("boom") }

	if err := editProvidersSection(config.Defaults()); err == nil {
		t.Fatal("expected error from failed confirm")
	}
}

func TestEditSections_Back(t *testing.T) {
	for name, edit := range map[string]func(*config.Config) error{
		"general":       editGeneralSection,
		"notifications": editNotificationsSection,
		"logging":       editLoggingSection,
	} {
		t.Run(name, func(t *testing.T) {
			scriptPrompts(t, []string{"Back"})
			if err := edit(config.Defaults()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestProviderEditors_PropagateSelectErrors(t *testing.T) {
	scriptPrompts(t, nil)
	selectOption = func(string, []string) (string, error) { return "", errors.New("boom") }
	var on bool
	var skip []string
	cfg := config.Defaults()

	for name, err := range map[string]error{
		"enabled-only": editEnabledOnlyProvider("Bun", &on),
		"enabled-skip": editEnabledSkipProvider("MacPorts", "Skip ports", &on, &skip),
		"uv":           editUvProvider(cfg),
		"providers":    editProvidersSection(cfg),
	} {
		if err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestEditStringSlice_RemoveItem(t *testing.T) {
	scriptPrompts(t, []string{"Remove item", "b", "Back"})

	got, err := editStringSlice("Skip", []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"a"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestEditMapStringBool_ToggleAndRemove(t *testing.T) {
	scriptPrompts(t, []string{"Toggle entry", "a", "Remove entry", "b", "Back"})

	got, err := editMapStringBool("Overrides", map[string]bool{"a": true, "b": true})
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]bool{"a": false}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}
