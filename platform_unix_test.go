//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// finderOff writes a symbolic hotkeys file with 65 turned off.
func finderOff(t *testing.T, app *App) {
	t.Helper()

	off := `<key>65</key><dict><key>enabled</key><false/></dict>`
	if err := os.MkdirAll(filepath.Dir(app.symbolicHotkeys), 0o700); err != nil {
		t.Fatal(err)
	}

	err := os.WriteFile(app.symbolicHotkeys, symbolicPrefs(off), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}
