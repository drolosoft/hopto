//go:build !darwin && !windows

package native

import "github.com/drolosoft/hopto/internal/platform"

// Stubs for systems without a native layer, so go vet and the pure tests
// run there. Nothing shows a window here.

// BecomeAccessory has no Dock or taskbar to leave.
func BecomeAccessory() {}

// RegisterHotkeys registers nothing: there is no system to hold the
// shortcuts.
func RegisterHotkeys(_, _ platform.Hotkey) {}

// CenterWindow has no window to move.
func CenterWindow(_ string, _ uint32) {}

// CurrentDisplay knows no display; 0 means none.
func CurrentDisplay() uint32 {
	return 0
}

// ActiveDisplays lists none.
func ActiveDisplays() []uint32 {
	return nil
}

// HandleReopen has no application delegate to hook.
func HandleReopen() {}

// Activate has nothing to bring forward.
func Activate() {}
