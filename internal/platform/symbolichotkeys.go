//go:build !windows

package platform

import (
	"math"
	"os"

	"howett.net/plist"

	"github.com/drolosoft/hopto/internal/library"
)

// SymbolicHotkeysFile, under the home folder, is where macOS keeps its
// own shortcuts (System Settings → Keyboard → Keyboard Shortcuts).
const SymbolicHotkeysFile = "Library/Preferences/" +
	"com.apple.symbolichotkeys.plist"

// finderSearchHotkey is the entry of "Show Finder search window", which
// comes with ⌘⌥Space: while it is on, the press never reaches hopto.
const finderSearchHotkey = "65"

// finderSearchCombo is ⌘⌥Space as a Hotkey, to compare with ours.
var finderSearchCombo = Hotkey{
	KeyCode:   keyCodes["space"],
	Modifiers: modifierCmd | modifierOption,
}

// finderSearchKeyCode is Space as macOS writes it in the entry's
// "value.parameters" triple, which uses NSEvent's numbering rather than
// the Carbon tables of keycodes_unix.go. Space happens to be 49 in both,
// but it is NSEvent's value, so it does not come from keyCodes.
const finderSearchKeyCode = 49

// finderSearchModifierCmd is NSEvent's Command bit in the same triple;
// Carbon's cmdKey is a different bit.
const finderSearchModifierCmd = 1 << 20

// finderSearchModifierOption is NSEvent's Option bit in the same triple.
// The entry collides with hopto only while both bits are set: a user can
// leave it on but move it away from cmd or option.
const finderSearchModifierOption = 1 << 19

// finderSearchShortcutOn reports whether the file still gives ⌘⌥Space to
// "Show Finder search window". macOS only writes an entry once the user
// changes it, so a missing entry (or a file that does not parse) is the
// factory setting, which is on. The "enabled" flag stays on when the user
// rebinds the shortcut to something else, so the entry's own combo is
// read too; when that triple is missing or does not parse, the flag
// decides alone.
func finderSearchShortcutOn(data []byte) bool {
	var prefs struct {
		Hotkeys map[string]map[string]any `plist:"AppleSymbolicHotKeys"`
	}

	if _, err := plist.Unmarshal(data, &prefs); err != nil {
		return true
	}

	entry, ok := prefs.Hotkeys[finderSearchHotkey]
	if !ok {
		return true
	}

	enabled, known := symbolicHotkeyFlag(entry)
	if !known {
		return true
	}

	if !enabled {
		return false
	}

	params, ok := symbolicHotkeyParameters(entry)
	if !ok {
		return true
	}

	return params.KeyCode == finderSearchKeyCode &&
		params.Modifiers&finderSearchModifierCmd != 0 &&
		params.Modifiers&finderSearchModifierOption != 0
}

// symbolicHotkeyFlag reads an entry's "enabled" flag: a boolean in the
// files seen so far, but an integer is accepted too, since older systems
// wrote 0 and 1. known is false for anything else, so the caller treats
// the entry as the factory setting.
func symbolicHotkeyFlag(entry map[string]any) (enabled, known bool) {
	switch flag := entry["enabled"].(type) {
	case bool:
		return flag, true
	case uint64:
		return flag != 0, true
	case int64:
		return flag != 0, true
	default:
		return false, false
	}
}

// symbolicCombo is a symbolic hotkey's own [keycode, modifiers], read
// from "value.parameters". Its modifiers are NSEvent's bits, not
// Carbon's masks, so it is a distinct type from Hotkey rather than one
// that could be compared against a Carbon-registered shortcut by
// mistake.
type symbolicCombo struct {
	// KeyCode is the physical key, the triple's second number.
	KeyCode uint32

	// Modifiers is NSEvent's modifier flags, the triple's third number.
	Modifiers uint32
}

// symbolicHotkeyParameters reads an entry's own combo, three numbers
// deep under "value.parameters": the physical key and the modifiers the
// user last bound it to. ok is false when the shape is not the one
// macOS is known to write.
func symbolicHotkeyParameters(entry map[string]any) (symbolicCombo, bool) {
	value, ok := entry["value"].(map[string]any)
	if !ok {
		return symbolicCombo{}, false
	}

	raw, ok := value["parameters"].([]any)
	if !ok || len(raw) < 3 {
		return symbolicCombo{}, false
	}

	keyCode, ok := symbolicHotkeyNumber(raw[1])
	if !ok {
		return symbolicCombo{}, false
	}

	modifiers, ok := symbolicHotkeyNumber(raw[2])
	if !ok {
		return symbolicCombo{}, false
	}

	return symbolicCombo{KeyCode: keyCode, Modifiers: modifiers}, true
}

// symbolicHotkeyNumber reads one number of a parameters triple. The
// plist library decodes <integer> as one of Go's signed or unsigned
// 64-bit kinds depending on its sign, and <real> as a float; a real
// counts only when it holds a whole number that fits.
func symbolicHotkeyNumber(value any) (uint32, bool) {
	switch number := value.(type) {
	case uint64:
		return uint32(number), true
	case int64:
		return uint32(number), true
	case float64:
		return wholeUint32(number)
	case float32:
		return wholeUint32(float64(number))
	default:
		return 0, false
	}
}

// wholeUint32 converts a float that holds a whole number between 0 and
// the largest uint32 (NaN is never equal to itself, so it fails too);
// anything else is not a number macOS writes for a key.
func wholeUint32(number float64) (uint32, bool) {
	whole := number == math.Trunc(number)
	if number < 0 || number > math.MaxUint32 || !whole {
		return 0, false
	}

	return uint32(number), true
}

// finderSearchShortcutEnabled reads the user's file and hands it to
// finderSearchShortcutOn; no file is the factory setting.
func finderSearchShortcutEnabled(path string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return true
	}

	return finderSearchShortcutOn(data)
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
