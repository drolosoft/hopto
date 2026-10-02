//go:build !darwin && !windows

package main

// statusBar has nothing to draw outside macOS; it lets `go vet` and the
// pure tests run elsewhere, like hotkey_other.go.
type statusBar struct{}

// Install does nothing here.
func (statusBar) Install(items []menuItem) {}

// SetChecked does nothing here.
func (statusBar) SetChecked(tag int, on bool) {}
