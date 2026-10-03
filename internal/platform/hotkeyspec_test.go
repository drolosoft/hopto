//go:build !windows

package platform

// These tests check Carbon key codes and the Command shortcuts of the
// macOS seed, so they run everywhere but Windows; the Windows table is
// checked by keycodes_windows_test.go.

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

	for _, test := range cases {
		got, err := ParseHotkey(test.spec)
		if err != nil {
			t.Errorf("%q: %v", test.spec, err)
			continue
		}

		if got != test.want {
			t.Errorf("%q = %+v, want %+v", test.spec, got, test.want)
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

// The system hands back the hotkey id with each press; an id hopto did
// not register opens the apps tab rather than nothing.
func TestTabForHotkey(t *testing.T) {
	cases := map[uint32]string{
		1: library.TabApps,
		2: library.TabLinks,
		0: library.TabApps,
		7: library.TabApps,
	}

	for id, want := range cases {
		if got := TabForHotkey(id); got != want {
			t.Errorf("TabForHotkey(%d) = %q, want %q", id, got, want)
		}
	}
}

// Shortcuts come from the library; a bad one falls back to the default
// with a log line, and two equal ones cannot both be registered.
func TestHotkeysFromSettings(t *testing.T) {
	custom := library.Default().Settings
	custom.HotkeyApps = "ctrl+option+a"
	custom.HotkeyLinks = "ctrl+option+l"

	apps, links := HotkeysFromSettings(custom)
	if apps.KeyCode != 0 || links.KeyCode != 37 {
		t.Errorf("custom: apps %+v links %+v", apps, links)
	}

	broken := library.Default().Settings
	broken.HotkeyApps = "cmd+space"

	apps, links = HotkeysFromSettings(broken)
	wrong := apps != mustHotkey(DefaultAppsHotkey) ||
		links != mustHotkey(DefaultLinksHotkey)
	if wrong {
		t.Errorf("broken: apps %+v links %+v", apps, links)
	}

	same := library.Default().Settings
	same.HotkeyApps = "cmd+option+space"
	same.HotkeyLinks = "cmd+option+space"

	apps, links = HotkeysFromSettings(same)
	if apps == links {
		t.Errorf("same: both shortcuts are %+v", apps)
	}
}
