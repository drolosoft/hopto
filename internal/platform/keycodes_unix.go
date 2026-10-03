//go:build !windows

package platform

import "runtime"

// The shortcut tables of macOS. keycodes_windows.go holds the same names
// in Windows' terms, so ParseHotkey and its tests compile on both.

// Name is what the page gets as `platform`; Linux says linux,
// which the page treats like macOS until it has tables of its own.
const Name = runtime.GOOS

// HotkeyExistsStatus is Carbon's eventHotKeyExistsErr: another app
// registered the same combination first.
const HotkeyExistsStatus int32 = -9878

// Modifier masks from Carbon's Events.h (cmdKey, shiftKey, optionKey,
// controlKey). They live here as plain numbers so this file has no cgo and
// the parser can be tested anywhere.
const (
	modifierCmd     uint32 = 0x0100
	modifierShift   uint32 = 0x0200
	modifierOption  uint32 = 0x0800
	modifierControl uint32 = 0x1000
)

// The shortcuts hopto uses out of the box. Cmd+Shift+Space is free on a
// stock macOS (Option+Space belongs to Alfred or Raycast, Cmd+Space to
// Spotlight). Cmd+Option+Space is "Show Finder search window" in macOS and
// the user has to disable that for the links shortcut to arrive.
const (
	DefaultAppsHotkey  = "cmd+shift+space"
	DefaultLinksHotkey = "cmd+option+space"
)

// modifierNames maps the words of a spec to their masks. "ctrl" and
// "control", "opt", "alt" and "option" are the same key.
var modifierNames = map[string]uint32{
	"cmd":     modifierCmd,
	"command": modifierCmd,
	"shift":   modifierShift,
	"option":  modifierOption,
	"opt":     modifierOption,
	"alt":     modifierOption,
	"ctrl":    modifierControl,
	"control": modifierControl,
}

// keyCodes are the virtual key codes of an ANSI keyboard (Carbon's
// Events.h, kVK_*). Letters and digits are positional: the code is the
// physical key, whatever the layout prints on it.
var keyCodes = map[string]uint32{
	"a": 0, "s": 1, "d": 2, "f": 3, "h": 4,
	"g": 5, "z": 6, "x": 7, "c": 8, "v": 9,
	"b": 11, "q": 12, "w": 13, "e": 14, "r": 15, "y": 16, "t": 17,
	"1": 18, "2": 19, "3": 20, "4": 21, "6": 22,
	"5": 23, "9": 25, "7": 26, "8": 28, "0": 29,
	"o": 31, "u": 32, "i": 34, "p": 35, "l": 37,
	"j": 38, "k": 40, "n": 45, "m": 46,
	"return": 36, "tab": 48, "space": 49, "escape": 53,
	"f1": 122, "f2": 120, "f3": 99, "f4": 118, "f5": 96, "f6": 97,
	"f7": 98, "f8": 100, "f9": 101, "f10": 109, "f11": 103, "f12": 111,
}

// reservedHotkeys are combinations macOS keeps for itself; Carbon would
// accept them and the app would never receive the press.
var reservedHotkeys = map[Hotkey]string{
	{KeyCode: 49, Modifiers: modifierCmd}: "Spotlight",
	{KeyCode: 48, Modifiers: modifierCmd}: "the app switcher",
}

// reservedBy words the refusal of a combination the system keeps.
func reservedBy(owner string) string {
	return "macOS keeps it for " + owner
}
