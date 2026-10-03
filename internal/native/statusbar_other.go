//go:build !darwin && !windows

package native

// StatusBar has nothing to draw outside macOS; it lets `go vet` and the
// pure tests run elsewhere, like hotkey_other.go.
type StatusBar struct{}

// Install does nothing here.
func (StatusBar) Install(items []MenuItem) {}

// SetChecked does nothing here.
func (StatusBar) SetChecked(tag int, on bool) {}
