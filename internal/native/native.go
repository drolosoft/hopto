// Package native is hopto's layer under Wails: Carbon hotkeys and Cocoa
// window placement on macOS, the Win32 window, tray and message loop on
// Windows, and the menu bar item or tray menu on both. It calls back into
// the launcher only through the Hooks handed to Start, and it imports
// neither Wails nor the launcher's own package.
package native

import (
	"log"
	"sync/atomic"

	"github.com/drolosoft/hopto/internal/platform"
)

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
	// Tag is what MenuPicked reports when the entry is chosen.
	Tag int
	// Label is the text of the entry, already translated by the caller.
	Label string
	// Checked draws the tick of an on/off entry.
	Checked bool
	// Separator makes the entry a line between groups.
	Separator bool
}

// hooks holds the callbacks the C handlers and the native thread reach.
// Start stores them once; those threads load them on every event, so the
// hand-over is atomic and does not depend on Start having returned before
// the first shortcut is registered. Until Start runs it holds nothing,
// and currentHooks answers with the empty ones.
var hooks atomic.Pointer[Hooks]

// Start keeps the launcher's hooks for the native side to call. It is
// called once, in startup, before the shortcuts are registered.
func Start(given Hooks) {
	filled := defaultHooks(given)
	hooks.Store(&filled)
}

// currentHooks returns the hooks Start stored, or the empty ones when an
// event comes in before Start: it is dropped instead of calling a nil
// func.
func currentHooks() Hooks {
	stored := hooks.Load()
	if stored == nil {
		return defaultHooks(Hooks{})
	}

	return *stored
}

// defaultHooks returns given with an empty function in place of every
// nil one, so the callers never have to check.
func defaultHooks(given Hooks) Hooks {
	if given.Toggle == nil {
		given.Toggle = func(string) {}
	}

	if given.Reopen == nil {
		given.Reopen = func() {}
	}

	if given.MenuPicked == nil {
		given.MenuPicked = func(int) {}
	}

	if given.HotkeyRegistered == nil {
		given.HotkeyRegistered = func(uint32, int32) {}
	}

	return given
}

// hotkeyPressed shows or hides the panel on the tab of the shortcut the
// system reported. The event arrives on the thread that owns the
// shortcuts (the main thread on macOS, the native thread on Windows), so
// the hook runs in a goroutine of its own and that thread never waits
// on the Wails runtime.
func hotkeyPressed(id uint32) {
	tab := platform.TabForHotkey(id)

	log.Printf("hotkey %d pressed: %s", id, tab)

	go currentHooks().Toggle(tab)
}

// hotkeyRegistered passes on what the system said to one shortcut and
// leaves it in the log. call is the system function that answered
// (RegisterEventHotKey or RegisterHotKey), named when it refused.
func hotkeyRegistered(id uint32, status int32, call string) {
	currentHooks().HotkeyRegistered(id, status)

	if status != 0 {
		log.Printf("hotkey %d: %s failed with status %d", id, call, status)
		return
	}

	log.Printf("hotkey %d registered", id)
}

// menuPicked hands the tag of the chosen menu entry to the launcher. The
// click arrives on the thread that drew the menu, which must not wait
// on the Wails runtime either, so the hook gets a goroutine too.
func menuPicked(tag int) {
	log.Printf("menu %d picked", tag)

	go currentHooks().MenuPicked(tag)
}
