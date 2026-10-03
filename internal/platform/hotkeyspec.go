package platform

import (
	"fmt"
	"log"
	"strings"

	"github.com/drolosoft/hopto/internal/library"
)

// Hotkey is a global shortcut in the terms Carbon's RegisterEventHotKey or
// Windows' RegisterHotKey wants: a virtual key code and a modifier mask.
type Hotkey struct {
	// KeyCode is the system's virtual key code for the key, from keyCodes.
	KeyCode uint32

	// Modifiers is the system's own modifier mask; the bits differ
	// between macOS and Windows, so a Hotkey never crosses systems.
	Modifiers uint32
}

// The hotkey ids hopto registers; the system hands them back with each
// press, so the handler knows which tab to open.
const (
	HotkeyApps  = 1
	HotkeyLinks = 2
)

// modifierNames maps the words of a spec to the masks each system defines
// in its keycodes file. "ctrl" and "control", "opt", "alt" and "option"
// are the same key; on Windows "cmd" and "command" are the Win key, which
// sits where Command does.
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

// ParseHotkey turns "cmd+shift+space" into a Hotkey. At least one of cmd,
// option or control is required: shift alone or a bare key would steal
// ordinary typing from every app.
func ParseHotkey(spec string) (Hotkey, error) {
	parts := strings.Split(strings.ToLower(spec), "+")
	if len(parts) < 2 {
		return Hotkey{}, fmt.Errorf(
			"hotkey %q: expected modifiers and a key, like cmd+shift+space", spec,
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
		return Hotkey{}, fmt.Errorf("hotkey %q: %s", spec, reservedBy(owner))
	}

	return hotkey, nil
}

// reservedBy is the clause that completes the refusal of a combination
// the system keeps for owner.
func reservedBy(owner string) string {
	return systemName + " keeps it for " + owner
}

// TabForHotkey maps the id the system hands back to the tab that shortcut
// opens. An id we did not register opens the apps tab: showing something is
// better than swallowing the press.
func TabForHotkey(id uint32) string {
	if id == HotkeyLinks {
		return library.TabLinks
	}

	return library.TabApps
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

// HotkeysFromSettings parses the two shortcuts of the library. A spec that
// does not parse falls back to its default with a log line, so a typo in
// the file never leaves hopto unreachable; two equal specs cannot both be
// registered, so the links one goes back to its default.
func HotkeysFromSettings(settings library.Settings) (apps, links Hotkey) {
	apps = parseOrDefault("hotkey_apps", settings.HotkeyApps, DefaultAppsHotkey)
	links = parseOrDefault(
		"hotkey_links", settings.HotkeyLinks, DefaultLinksHotkey,
	)

	if apps == links {
		log.Printf(
			"hotkeys: hotkey_apps and hotkey_links are both %q, using the defaults",
			settings.HotkeyLinks,
		)
		links = mustHotkey(DefaultLinksHotkey)

		if apps == links {
			apps = mustHotkey(DefaultAppsHotkey)
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
