package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/drolosoft/hopto/internal/library"
)

// Hotkey is a global shortcut in the terms Carbon's RegisterEventHotKey
// wants: a virtual key code and a mask of modifiers.
type Hotkey struct {
	KeyCode   uint32
	Modifiers uint32
}

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
	defaultAppsHotkey  = "cmd+shift+space"
	defaultLinksHotkey = "cmd+option+space"
)

// The hot key ids handed to Carbon; they come back in the event so the
// handler knows which tab to open.
const (
	hotkeyApps  = 1
	hotkeyLinks = 2
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

// ParseHotkey turns "cmd+shift+space" into a Hotkey. At least one of cmd,
// option or control is required: shift alone or a bare key would steal
// ordinary typing from every app.
func ParseHotkey(spec string) (Hotkey, error) {
	parts := strings.Split(strings.ToLower(spec), "+")
	if len(parts) < 2 {
		return Hotkey{}, fmt.Errorf(
			"hotkey %q: expected modifiers and a key, like cmd+shift+space",
			spec,
		)
	}

	var hotkey Hotkey

	for _, part := range parts[:len(parts)-1] {
		trimmed := strings.TrimSpace(part)
		mask, ok := modifierNames[trimmed]
		if !ok {
			return Hotkey{}, fmt.Errorf(
				"hotkey %q: unknown modifier %q", spec, trimmed,
			)
		}

		hotkey.Modifiers |= mask
	}

	key := strings.TrimSpace(parts[len(parts)-1])
	code, ok := keyCodes[key]
	if !ok {
		return Hotkey{}, fmt.Errorf("hotkey %q: unknown key %q", spec, key)
	}

	hotkey.KeyCode = code

	if hotkey.Modifiers&(modifierCmd|modifierOption|modifierControl) == 0 {
		return Hotkey{}, fmt.Errorf("hotkey %q: needs cmd, option or control", spec)
	}

	if owner, reserved := reservedHotkeys[hotkey]; reserved {
		return Hotkey{}, fmt.Errorf("hotkey %q: macOS keeps it for %s", spec, owner)
	}

	return hotkey, nil
}

// tabForHotkey maps the id Carbon hands back to the tab that shortcut
// opens. An id we did not register opens the apps tab: showing something is
// better than swallowing the press.
func tabForHotkey(id uint32) string {
	if id == hotkeyLinks {
		return tabLinks
	}

	return tabApps
}

// mustHotkey parses a built-in spec; a typo there is a programming error,
// so it panics rather than starting without a shortcut.
func mustHotkey(spec string) Hotkey {
	hotkey, err := ParseHotkey(spec)
	if err != nil {
		panic(err)
	}

	return hotkey
}

// hotkeysFromSettings parses the two shortcuts of the library. A spec that
// does not parse falls back to its default with a log line, so a typo in
// the file never leaves hopto unreachable; two equal specs cannot both be
// registered, so the links one goes back to its default.
func hotkeysFromSettings(settings library.Settings) (apps, links Hotkey) {
	apps = parseOrDefault("hotkey_apps", settings.HotkeyApps, defaultAppsHotkey)
	links = parseOrDefault(
		"hotkey_links", settings.HotkeyLinks, defaultLinksHotkey,
	)

	if apps == links {
		log.Printf(
			"hotkeys: hotkey_apps and hotkey_links are both %q, using the defaults",
			settings.HotkeyLinks,
		)
		links = mustHotkey(defaultLinksHotkey)

		if apps == links {
			apps = mustHotkey(defaultAppsHotkey)
		}
	}

	return apps, links
}

// parseOrDefault is one shortcut with its fallback.
func parseOrDefault(field, spec, fallback string) Hotkey {
	hotkey, err := ParseHotkey(spec)
	if err != nil {
		log.Printf("hotkeys: %s: %v, using %s", field, err, fallback)
		return mustHotkey(fallback)
	}

	return hotkey
}
