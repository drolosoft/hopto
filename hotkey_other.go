//go:build !darwin && !windows

package main

// The native side only exists on macOS; these stubs let `go vet` and the
// pure tests run elsewhere. Nothing shows a window here.
func becomeAccessory() {}

// registerToggleHotkeys registers nothing: global shortcuts are Carbon's.
func registerToggleHotkeys(toggle func(tab string), apps, links Hotkey) {}

// centerWindow has no window to move.
func centerWindow(mode string, display uint32) {}

// currentDisplay knows no display; 0 means none.
func currentDisplay() uint32 { return 0 }

// activeDisplays lists none.
func activeDisplays() []uint32 { return nil }

// handleReopen has no application delegate to hook.
func handleReopen(show func()) {}

// shutdownNative has no tray icon to remove.
func shutdownNative() {}

// activateApp has nothing to bring forward.
func activateApp() {}
