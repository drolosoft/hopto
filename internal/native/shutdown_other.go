//go:build !windows

package native

// Shutdown has nothing to undo: the menu bar item goes away with the
// process on macOS, and other systems have no native layer at all. Only
// Windows has to remove its tray icon by hand.
func Shutdown() {}
