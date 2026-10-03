// Package native is hopto's layer under Wails: Carbon hotkeys and Cocoa
// window placement on macOS, the Win32 window, tray and message loop on
// Windows, and the menu bar item or tray menu on both. It calls back into
// the launcher only through the Hooks handed to Start, and it imports
// neither Wails nor the launcher's own package.
package native

// Hooks is what the native layer calls back into: the launcher hands one
// to Start once, in startup. Every field may be nil; a nil hook does
// nothing, because a shortcut can arrive before the launcher is ready to
// hear it.
type Hooks struct {
	// Toggle shows or hides the panel on the tab the shortcut names.
	Toggle func(tab string)
	// Reopen runs when the Dock or a second launch asks for the panel.
	Reopen func()
	// MenuPicked runs with the tag of the menu bar or tray entry chosen.
	MenuPicked func(tag int)
	// HotkeyRegistered reports what the system said to each shortcut.
	HotkeyRegistered func(id uint32, status int32)
}

// MenuItem is one entry of the menu bar item or the tray menu. A
// Separator has no label and no tag.
type MenuItem struct {
	Tag       int
	Label     string
	Checked   bool
	Separator bool
}

// hooks are the callbacks the C handlers and the native thread reach.
// Start fills them once; until then they are the empty ones, so an
// event that comes in first is dropped instead of calling a nil func.
var hooks = defaultHooks(Hooks{})

// Start keeps the launcher's hooks for the native side to call. It is
// called once, in startup, before the shortcuts are registered.
func Start(h Hooks) {
	hooks = defaultHooks(h)
}

// defaultHooks returns h with an empty function in place of every nil
// one, so the callers never have to check.
func defaultHooks(h Hooks) Hooks {
	if h.Toggle == nil {
		h.Toggle = func(tab string) {}
	}

	if h.Reopen == nil {
		h.Reopen = func() {}
	}

	if h.MenuPicked == nil {
		h.MenuPicked = func(tag int) {}
	}

	if h.HotkeyRegistered == nil {
		h.HotkeyRegistered = func(id uint32, status int32) {}
	}

	return h
}
