//go:build darwin

package main

// shutdownNative has nothing to undo: the status item goes away with the
// process on macOS, so only Windows has to remove its tray icon by hand.
func shutdownNative() {}
