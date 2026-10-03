package native

import (
	"testing"

	"github.com/drolosoft/hopto/internal/library"
	"github.com/drolosoft/hopto/internal/platform"
)

// TestNilHooksDoNothing starts the package with no hooks at all, as a
// shortcut pressed before the launcher listens would find it, and calls
// each one: an empty function must stand in for every nil.
func TestNilHooksDoNothing(t *testing.T) {
	Start(Hooks{})

	// None of these may panic.
	hooks.Toggle(library.TabApps)
	hooks.Reopen()
	hooks.MenuPicked(1)
	hooks.HotkeyRegistered(platform.HotkeyApps, 0)
}
