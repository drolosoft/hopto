//go:build windows

// The two global hotkeys on Windows: registering them on the native
// thread and reporting what Windows said about each one.

package native

import (
	"errors"

	"github.com/drolosoft/hopto/internal/platform"
	"golang.org/x/sys/windows"
)

// RegisterHotkeys binds the apps shortcut to the apps tab and the links
// shortcut to the links tab, on the native thread; a press reaches
// hooks.Toggle.
func RegisterHotkeys(apps, links platform.Hotkey) {
	registerHotkey(platform.HotkeyApps, apps)
	registerHotkey(platform.HotkeyLinks, links)
}

// registerHotkey registers one shortcut on the native thread and records
// what Windows said, so the launcher can show a taken combination. A
// held key fires once.
func registerHotkey(id uint32, hotkey platform.Hotkey) {
	runNative(func() {
		ok, _, err := procRegisterHotKey.Call(
			uintptr(thread.window), uintptr(id),
			uintptr(hotkey.Modifiers|modNoRepeat), uintptr(hotkey.KeyCode),
		)

		status := int32(0)
		if ok == 0 {
			status = failureStatus(err)
		}

		hotkeyRegistered(id, status, "RegisterHotKey")
	})
}

// failureStatus is the status reported for a refused registration: the
// Windows error code, or -1 when the call left none. A failure must
// never read as success, which is what status 0 means to the caller.
func failureStatus(err error) int32 {
	errno, isErrno := errors.AsType[windows.Errno](err)
	if !isErrno || errno == 0 {
		return -1
	}

	return int32(errno)
}
