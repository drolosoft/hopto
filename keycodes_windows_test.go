//go:build windows

package main

import (
	"strings"
	"testing"
)

// Review Focus 4: a spec copied from the macOS docs registers the Win key
// on Windows, and Win+Shift+Space is the input language switch.
func TestWindowsReservedHotkeys(t *testing.T) {
	_, err := ParseHotkey("cmd+shift+space")
	if err == nil || !strings.Contains(err.Error(), "Windows keeps it") {
		t.Fatalf("cmd+shift+space: want the reserved error, got %v", err)
	}

	hotkey, err := ParseHotkey("ctrl+shift+space")
	if err != nil {
		t.Fatal(err)
	}

	want := Hotkey{KeyCode: 0x20, Modifiers: modifierControl | modifierShift}
	if hotkey != want {
		t.Errorf("ctrl+shift+space: got %+v, want %+v", hotkey, want)
	}
}

// The defaults of the Windows seed must parse with the Windows tables.
func TestWindowsDefaultsParse(t *testing.T) {
	for _, spec := range []string{defaultAppsHotkey, defaultLinksHotkey} {
		if _, err := ParseHotkey(spec); err != nil {
			t.Errorf("%s: %v", spec, err)
		}
	}

	if defaultAppsHotkey != "ctrl+shift+space" {
		t.Errorf("apps default is %q", defaultAppsHotkey)
	}
}
