//go:build windows

package platform

// The shortcut tables in Windows' terms: RegisterHotKey wants a virtual
// key code and a MOD_* mask. Same names as keycodes_unix.go.

// Name is what the page gets as `platform`.
const Name = "windows"

// systemName is how a refusal names the system that keeps a combination.
const systemName = "Windows"

// HotkeyExistsStatus is ERROR_HOTKEY_ALREADY_REGISTERED (1409): another
// app registered the same combination first.
const HotkeyExistsStatus int32 = 1409

// Modifier masks of RegisterHotKey (winuser.h: MOD_ALT, MOD_CONTROL,
// MOD_SHIFT, MOD_WIN). "cmd" is the Win key: it sits where Command does.
const (
	modifierOption  uint32 = 0x0001
	modifierControl uint32 = 0x0002
	modifierShift   uint32 = 0x0004
	modifierCmd     uint32 = 0x0008
)

// The shortcuts hopto uses out of the box on Windows. Win+Space combos
// switch the input language and Alt+Space opens a window's system menu;
// Ctrl+Shift+Space and Ctrl+Alt+Space are free on a stock system.
const (
	DefaultAppsHotkey  = "ctrl+shift+space"
	DefaultLinksHotkey = "ctrl+alt+space"
)

// keyCodes are virtual key codes (winuser.h, VK_*). Letters and digits
// are their ASCII upper-case values; the named keys are VK_RETURN,
// VK_TAB, VK_SPACE, VK_ESCAPE and VK_F1 to VK_F12.
var keyCodes = map[string]uint32{
	"a": 0x41, "b": 0x42, "c": 0x43, "d": 0x44, "e": 0x45, "f": 0x46,
	"g": 0x47, "h": 0x48, "i": 0x49, "j": 0x4A, "k": 0x4B, "l": 0x4C,
	"m": 0x4D, "n": 0x4E, "o": 0x4F, "p": 0x50, "q": 0x51, "r": 0x52,
	"s": 0x53, "t": 0x54, "u": 0x55, "v": 0x56, "w": 0x57, "x": 0x58,
	"y": 0x59, "z": 0x5A,
	"0": 0x30, "1": 0x31, "2": 0x32, "3": 0x33, "4": 0x34,
	"5": 0x35, "6": 0x36, "7": 0x37, "8": 0x38, "9": 0x39,
	"return": 0x0D, "tab": 0x09, "space": 0x20, "escape": 0x1B,
	"f1": 0x70, "f2": 0x71, "f3": 0x72, "f4": 0x73, "f5": 0x74,
	"f6": 0x75, "f7": 0x76, "f8": 0x77, "f9": 0x78, "f10": 0x79,
	"f11": 0x7A, "f12": 0x7B,
}

// reservedHotkeys are combinations Windows keeps for itself:
// RegisterHotKey accepts them and the press never arrives.
var reservedHotkeys = map[Hotkey]string{
	{KeyCode: keyCodes["l"], Modifiers: modifierCmd}:   "locking the session",
	{KeyCode: keyCodes["tab"], Modifiers: modifierCmd}: "Task View",
	{
		KeyCode:   keyCodes["space"],
		Modifiers: modifierCmd,
	}: "the input language switch",
	{
		KeyCode:   keyCodes["space"],
		Modifiers: modifierCmd | modifierShift,
	}: "the input language switch",
}
