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

	current := currentHooks()

	// None of these may panic.
	current.Toggle(library.TabApps)
	current.Reopen()
	current.MenuPicked(1)
	current.HotkeyRegistered(platform.HotkeyApps, 0)
}

// TestEventsBeforeStartAreDropped clears whatever an earlier test stored
// and sends each event helper an event, as the first shortcut would
// arrive before the launcher has called Start: none may panic.
func TestEventsBeforeStartAreDropped(t *testing.T) {
	hooks.Store(nil)
	t.Cleanup(func() { hooks.Store(nil) })

	hotkeyPressed(platform.HotkeyApps)
	hotkeyRegistered(platform.HotkeyApps, 0, "RegisterHotKey")
	hotkeyRegistered(platform.HotkeyApps, 1, "RegisterHotKey")
	menuPicked(1)

	// The reopen hook only has a caller on macOS, so it is read here.
	currentHooks().Reopen()
}
