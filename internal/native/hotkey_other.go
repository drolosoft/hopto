//go:build !darwin && !windows

package native

import "github.com/drolosoft/hopto/internal/platform"

// The native side only exists on macOS; these stubs let `go vet` and the
// pure tests run elsewhere. Nothing shows a window here.
func BecomeAccessory() {}

// RegisterHotkeys registers nothing: global shortcuts are Carbon's.
func RegisterHotkeys(apps, links platform.Hotkey) {}

// CenterWindow has no window to move.
func CenterWindow(mode string, display uint32) {}

// CurrentDisplay knows no display; 0 means none.
func CurrentDisplay() uint32 { return 0 }

// ActiveDisplays lists none.
func ActiveDisplays() []uint32 { return nil }

// HandleReopen has no application delegate to hook.
func HandleReopen() {}

// Shutdown has no tray icon to remove.
func Shutdown() {}

// Activate has nothing to bring forward.
func Activate() {}
