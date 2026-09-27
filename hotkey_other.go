//go:build !darwin

package main

// The native side only exists on macOS; these stubs let `go vet` and the
// pure tests run elsewhere. Nothing shows a window here.
func becomeAccessory() {}

func registerToggleHotkeys(toggle func(tab string), apps, links Hotkey) {}

func centerOnActiveScreen() {}

func activateApp() {}
