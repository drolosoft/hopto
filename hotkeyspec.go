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

// windowClassName is the Win32 class of hopto's window, set through
// Wails' options so the native side can find the window by name. macOS
// ignores it.
const windowClassName = "hoptoWindow"

// The hot key ids handed to Carbon; they come back in the event so the
// handler knows which tab to open.
const (
	hotkeyApps  = 1
	hotkeyLinks = 2
)

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
		return Hotkey{}, fmt.Errorf(
			"hotkey %q: %s", spec, reservedBy(owner),
		)
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
