//go:build windows

package seed

// The Windows defaults, the same strings as keycodes_windows.go in the
// main package (a test there checks they agree): Win+Space combos switch
// the input language and Alt+Space opens a window's system menu.
const (
	windowsAppsHotkey  = "ctrl+shift+space"
	windowsLinksHotkey = "ctrl+alt+space"
)

// platformSeed rewrites the shortcut lines for Windows.
func platformSeed(data []byte) []byte {
	return withHotkeys(data, windowsAppsHotkey, windowsLinksHotkey)
}
