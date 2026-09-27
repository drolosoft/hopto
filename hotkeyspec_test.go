package main

import (
	"testing"

	"github.com/drolosoft/hopto/internal/library"
)

// The spec strings the library accepts, and the Carbon values they mean.
func TestParseHotkey(t *testing.T) {
	cases := []struct {
		spec string
		want Hotkey
	}{
		{
			"cmd+shift+space",
			Hotkey{KeyCode: 49, Modifiers: modifierCmd | modifierShift},
		},
		{
			"cmd+option+space",
			Hotkey{KeyCode: 49, Modifiers: modifierCmd | modifierOption},
		},
		{
			" Cmd + Shift + Space ",
			Hotkey{KeyCode: 49, Modifiers: modifierCmd | modifierShift},
		},
		{
			"ctrl+option+l",
			Hotkey{KeyCode: 37, Modifiers: modifierControl | modifierOption},
		},
		{"cmd+f12", Hotkey{KeyCode: 111, Modifiers: modifierCmd}},
		{"option+1", Hotkey{KeyCode: 18, Modifiers: modifierOption}},
	}

	for _, tc := range cases {
		got, err := ParseHotkey(tc.spec)
		if err != nil {
			t.Errorf("%q: %v", tc.spec, err)
			continue
		}

		if got != tc.want {
			t.Errorf("%q = %+v, want %+v", tc.spec, got, tc.want)
		}
	}
}

// Specs that would hijack typing or that macOS keeps for itself are
// refused with the spec in the message, so the log says which line of the
// library is wrong.
func TestParseHotkeyRejects(t *testing.T) {
	specs := []string{
		"", "space", "shift+space", "cmd+space", "cmd+tab", "cmd+",
		"cmd+shift+", "cmd+shift+nosuchkey", "super+space",
		"cmd+shift+space+a",
	}
	for _, spec := range specs {
		if _, err := ParseHotkey(spec); err == nil {
			t.Errorf("%q: expected an error", spec)
		}
	}
}

// The hot key ids handed to Carbon come back in the event; anything
// unknown opens the apps tab rather than nothing.
func TestTabForHotkey(t *testing.T) {
	cases := map[uint32]string{1: tabApps, 2: tabLinks, 0: tabApps, 7: tabApps}

	for id, want := range cases {
		if got := tabForHotkey(id); got != want {
			t.Errorf("tabForHotkey(%d) = %q, want %q", id, got, want)
		}
	}
}

// Shortcuts come from the library; a bad one falls back to the default
// with a log line, and two equal ones cannot both be registered.
func TestHotkeysFromSettings(t *testing.T) {
	custom := library.Default().Settings
	custom.HotkeyApps = "ctrl+option+a"
	custom.HotkeyLinks = "ctrl+option+l"

	apps, links := hotkeysFromSettings(custom)
	if apps.KeyCode != 0 || links.KeyCode != 37 {
		t.Errorf("custom: apps %+v links %+v", apps, links)
	}

	broken := library.Default().Settings
	broken.HotkeyApps = "cmd+space"

	apps, links = hotkeysFromSettings(broken)
	wrong := apps != mustHotkey(defaultAppsHotkey) ||
		links != mustHotkey(defaultLinksHotkey)
	if wrong {
		t.Errorf("broken: apps %+v links %+v", apps, links)
	}

	same := library.Default().Settings
	same.HotkeyApps = "cmd+option+space"
	same.HotkeyLinks = "cmd+option+space"

	apps, links = hotkeysFromSettings(same)
	if apps == links {
		t.Errorf("same: both shortcuts are %+v", apps)
	}
}
