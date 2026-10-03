//go:build !darwin && !windows

package native

// StatusBar has nothing to draw on systems with neither the menu bar nor
// the tray; it lets go vet and the pure tests run there, like
// hotkey_other.go.
type StatusBar struct{}

// Install does nothing here.
func (StatusBar) Install(_ []MenuItem) {}

// SetChecked does nothing here.
func (StatusBar) SetChecked(_ int, _ bool) {}
