//go:build !windows

package app

import (
	"os"
	"path/filepath"
	"testing"
)

// finderOff writes a symbolic hotkeys file with the Finder's shortcut
// (entry 65) turned off, so a test that is not about the Finder never
// finds the user's real one in the way. It is the smallest plist the
// platform reader accepts, written here because the helper that builds
// such files belongs to the platform package's own tests.
func finderOff(t *testing.T, app *App) {
	t.Helper()

	plist := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<plist version="1.0"><dict><key>AppleSymbolicHotKeys</key>` +
		`<dict><key>65</key><dict><key>enabled</key><false/></dict>` +
		`</dict></dict></plist>`

	err := os.MkdirAll(filepath.Dir(app.symbolicHotkeys), 0o700)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(app.symbolicHotkeys, []byte(plist), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}
