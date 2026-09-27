package main

import (
	"os"

	"howett.net/plist"

	"github.com/drolosoft/hopto/internal/library"
)

// symbolicHotkeysFile, under the home folder, is where macOS keeps its
// own shortcuts (System Settings → Keyboard → Keyboard Shortcuts).
const symbolicHotkeysFile = "Library/Preferences/" +
	"com.apple.symbolichotkeys.plist"

// finderSearchHotkey is the entry of "Show Finder search window", which
// comes with ⌘⌥Space: while it is on, the press never reaches hopto.
const finderSearchHotkey = "65"

// finderSearchCombo is ⌘⌥Space as a Hotkey, to compare with ours.
var finderSearchCombo = Hotkey{
	KeyCode:   keyCodes["space"],
	Modifiers: modifierCmd | modifierOption,
}

// symbolicHotkeyEnabled reads one entry of the file. macOS only writes
// an entry once the user changes it, so a missing entry (or a file that
// does not parse) is the factory setting, which is on.
func symbolicHotkeyEnabled(data []byte, id string) bool {
	var prefs struct {
		Hotkeys map[string]map[string]any `plist:"AppleSymbolicHotKeys"`
	}

	if _, err := plist.Unmarshal(data, &prefs); err != nil {
		return true
	}

	entry, ok := prefs.Hotkeys[id]
	if !ok {
		return true
	}

	// The flag is a boolean in the files seen so far; an integer is
	// accepted too, since older systems wrote 0 and 1.
	switch enabled := entry["enabled"].(type) {
	case bool:
		return enabled
	case uint64:
		return enabled != 0
	case int64:
		return enabled != 0
	default:
		return true
	}
}

// finderSearchShortcutEnabled reads the user's file; no file is the
// factory setting.
func finderSearchShortcutEnabled(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}

	return symbolicHotkeyEnabled(data, finderSearchHotkey)
}

// usesFinderShortcut reports whether either shortcut of the settings is
// ⌘⌥Space, whatever the order the modifiers were written in.
func usesFinderShortcut(settings library.Settings) bool {
	for _, spec := range []string{settings.HotkeyApps, settings.HotkeyLinks} {
		hotkey, err := ParseHotkey(spec)
		if err == nil && hotkey == finderSearchCombo {
			return true
		}
	}

	return false
}
