package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/drolosoft/hopto/internal/library"
)

// symbolicPrefs is the shape of com.apple.symbolichotkeys.plist, as XML
// (the real file is binary; the reader takes both).
func symbolicPrefs(entries string) []byte {
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>` +
		`<plist version="1.0"><dict><key>AppleSymbolicHotKeys</key>` +
		`<dict>` + entries + `</dict></dict></plist>`)
}

// macOS writes an entry only once the user touches it; a missing one is
// the factory setting, which for 65 is on.
func TestSymbolicHotkeyEnabled(t *testing.T) {
	off := `<key>65</key><dict><key>enabled</key><false/></dict>`
	on := `<key>65</key><dict><key>enabled</key><true/></dict>`
	other := `<key>64</key><dict><key>enabled</key><false/></dict>`

	cases := []struct {
		name string
		data []byte
		want bool
	}{
		{"turned off", symbolicPrefs(off), false},
		{"turned on", symbolicPrefs(on), true},
		{"never touched", symbolicPrefs(other), true},
		{"not a plist", []byte("garbage"), true},
		{"empty", nil, true},
	}

	for _, test := range cases {
		got := symbolicHotkeyEnabled(test.data, finderSearchHotkey)
		if got != test.want {
			t.Errorf("%s: got %v", test.name, got)
		}
	}
}

// The file is read from the path; no file is the factory setting.
func TestFinderSearchShortcutEnabled(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.plist")
	if !finderSearchShortcutEnabled(path) {
		t.Error("missing file must read as the factory setting, on")
	}

	off := `<key>65</key><dict><key>enabled</key><false/></dict>`
	if err := os.WriteFile(path, symbolicPrefs(off), 0o600); err != nil {
		t.Fatal(err)
	}

	if finderSearchShortcutEnabled(path) {
		t.Error("turned off, read as on")
	}
}

// Only ⌘⌥Space collides with the Finder's window, on either shortcut.
func TestUsesFinderShortcut(t *testing.T) {
	settings := library.Default().Settings
	if !usesFinderShortcut(settings) {
		t.Error("the default links shortcut is ⌘⌥Space")
	}

	settings.HotkeyLinks = "ctrl+option+space"
	if usesFinderShortcut(settings) {
		t.Error("ctrl+option+space is not the Finder's")
	}

	settings.HotkeyApps = "option+cmd+space"
	if !usesFinderShortcut(settings) {
		t.Error("the order of the modifiers does not matter")
	}
}
