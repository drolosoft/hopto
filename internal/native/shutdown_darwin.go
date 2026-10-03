//go:build darwin

package native

// Shutdown has nothing to undo: the status item goes away with the
// process on macOS, so only Windows has to remove its tray icon by hand.
func Shutdown() {}
