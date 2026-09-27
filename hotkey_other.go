//go:build !darwin

package main

// The native side only exists on macOS; these stubs let `go vet` and the
// pure tests run elsewhere. Nothing shows a window here.
func becomeAccessory() {}

func registerToggleHotkeys(toggle func(tab string), apps, links Hotkey) {}

func centerWindow(mode string, display uint32) {}

func currentDisplay() uint32 { return 0 }

func activeDisplays() []uint32 { return nil }

func handleReopen(show func()) {}

func activateApp() {}
