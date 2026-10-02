//go:build windows

package main

import (
	"hash/fnv"
	"log"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The window side of the native layer: find Wails' window, keep it off
// the taskbar, place it on a monitor and bring it to the front. The
// tray and the hotkeys are in native_windows.go.

var (
	procFindWindowW       = user32.NewProc("FindWindowW")
	procGetWindowLongPtrW = user32.NewProc("GetWindowLongPtrW")
	procSetWindowLongPtrW = user32.NewProc("SetWindowLongPtrW")
	procGetWindowRect     = user32.NewProc("GetWindowRect")
	procSetWindowPos      = user32.NewProc("SetWindowPos")
	procMonitorFromPoint  = user32.NewProc("MonitorFromPoint")
	procMonitorFromWindow = user32.NewProc("MonitorFromWindow")
	procGetMonitorInfoW   = user32.NewProc("GetMonitorInfoW")

	procEnumDisplayMonitors = user32.NewProc("EnumDisplayMonitors")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procAttachThreadInput   = user32.NewProc("AttachThreadInput")
	procBringWindowToTop    = user32.NewProc("BringWindowToTop")
	procSetFocus            = user32.NewProc("SetFocus")

	procGetWindowThreadProcessId = user32.NewProc(
		"GetWindowThreadProcessId",
	)
)

// Extended window styles (winuser.h). A tool window has no taskbar
// button and no Alt+Tab entry; WS_EX_APPWINDOW forces both, and Wails
// sets it, so it is cleared. GWL_EXSTYLE is -20, written as its
// two's complement because a negative constant does not fit a uintptr.
const (
	gwlExStyle     = ^uintptr(19)
	wsExToolWindow = 0x00000080
	wsExAppWindow  = 0x00040000
)

// SetWindowPos flags: keep the size and the z-order, do not activate.
const (
	swpNoSize     = 0x0001
	swpNoZOrder   = 0x0004
	swpNoActivate = 0x0010
)

// monitorDefaultToNearest is the MonitorFrom* fallback (winuser.h
// MONITOR_DEFAULTTONEAREST): a point outside every monitor gets the
// closest one.
const monitorDefaultToNearest = 0x2

// rect is RECT.
type rect struct {
	Left, Top, Right, Bottom int32
}

// monitorInfoEx is MONITORINFOEXW: the monitor's area, its work area
// (minus the taskbar) and its device name, which is what hopto hashes
// into a display id that survives a reboot (HMONITOR does not).
type monitorInfoEx struct {
	Size    uint32
	Monitor rect
	Work    rect
	Flags   uint32
	Device  [32]uint16
}

// onHotkey is what the native thread calls with the tab of the pressed
// shortcut; registerToggleHotkeys installs the App's toggle in it.
var onHotkey = func(tab string) {}

// launcherWindow is Wails' window, found by the class name main.go set.
func launcherWindow() windows.Handle {
	className, _ := windows.UTF16PtrFromString(windowClassName)
	window, _, _ := procFindWindowW.Call(
		uintptr(unsafe.Pointer(className)), 0,
	)

	return windows.Handle(window)
}

// becomeAccessory keeps the launcher off the taskbar and out of
// Alt+Tab: the Windows side of the accessory activation policy. Done
// while the window is still hidden, so the style change never flickers.
func becomeAccessory() {
	window := launcherWindow()
	if window == 0 {
		log.Printf("native: launcher window not found")
		return
	}

	style, _, _ := procGetWindowLongPtrW.Call(
		uintptr(window), gwlExStyle,
	)
	style = (style &^ wsExAppWindow) | wsExToolWindow
	_, _, _ = procSetWindowLongPtrW.Call(uintptr(window), gwlExStyle, style)
}

// registerToggleHotkeys binds the apps shortcut to the apps tab and the
// links shortcut to the links tab, on the native thread.
func registerToggleHotkeys(toggle func(tab string), apps, links Hotkey) {
	onHotkey = toggle
	registerHotkey(hotkeyApps, apps)
	registerHotkey(hotkeyLinks, links)
}

// displayID is the id hopto keeps for a monitor: FNV-32 of its device
// name (\\.\DISPLAY1), never 0 (0 means no display).
func displayID(info monitorInfoEx) uint32 {
	name := windows.UTF16ToString(info.Device[:])
	hash := fnv.New32a()
	hash.Write([]byte(name))

	id := hash.Sum32()
	if id == 0 {
		return 1
	}

	return id
}

// monitorInfo reads one monitor; ok is false when the handle is gone.
func monitorInfo(monitor uintptr) (monitorInfoEx, bool) {
	var info monitorInfoEx
	info.Size = uint32(unsafe.Sizeof(info))

	ok, _, _ := procGetMonitorInfoW.Call(
		monitor, uintptr(unsafe.Pointer(&info)),
	)

	return info, ok != 0
}

// monitorCallback is the MONITORENUMPROC handed to EnumDisplayMonitors.
// It is made once: the runtime keeps every callback it creates in a table
// of 2000 slots that is never freed, so creating one per call would crash
// the process after about a thousand shows.
var monitorCallback = windows.NewCallback(collectMonitor)

// enumeration is where collectMonitor puts what it finds. The callback
// cannot be a closure over a local (that is the one-callback-per-call
// problem again), and turning the data argument back into a pointer is
// what go vet rejects, so the list is shared and enumerationLock keeps
// two enumerations from mixing.
var (
	enumeration     []monitorInfoEx
	enumerationLock sync.Mutex
)

// collectMonitor appends one monitor to the enumeration in progress.
func collectMonitor(monitor, dc, area, data uintptr) uintptr {
	if info, ok := monitorInfo(monitor); ok {
		enumeration = append(enumeration, info)
	}

	// Non-zero continues the enumeration.
	return 1
}

// monitors lists the attached monitors with their info.
func monitors() []monitorInfoEx {
	enumerationLock.Lock()
	defer enumerationLock.Unlock()

	enumeration = nil
	_, _, _ = procEnumDisplayMonitors.Call(0, 0, monitorCallback, 0)

	return enumeration
}

// shutdownNative takes the tray icon away before the process exits:
// Wails ends with a WM_QUIT to its own thread, so the native window is
// never destroyed and its WM_DESTROY never arrives. Nothing to do when
// the native thread never started.
func shutdownNative() {
	if native.window == 0 {
		return
	}

	runNative(func() {
		removeTrayIcon(native.window)
	})
}

// chosenMonitor picks the monitor for this show: under the mouse, the
// remembered one, or the primary (the one whose work area starts at
// 0,0; the enumeration lists it first).
func chosenMonitor(mode string, display uint32) (monitorInfoEx, bool) {
	if mode == screenMouse {
		var cursor point
		_, _, _ = procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))

		// MonitorFromPoint takes the POINT by value, and on amd64 and
		// arm64 an 8-byte struct travels in one register: X in the low
		// half, Y in the high half. Passed as two arguments, Windows
		// would read (X, 0) and take Y for the flags.
		packed := uintptr(uint32(cursor.X)) | uintptr(uint32(cursor.Y))<<32
		monitor, _, _ := procMonitorFromPoint.Call(
			packed, monitorDefaultToNearest,
		)

		return monitorInfo(monitor)
	}

	all := monitors()

	if mode == screenLast {
		for _, info := range all {
			if displayID(info) == display {
				return info, true
			}
		}
	}

	for _, info := range all {
		if info.Monitor.Left == 0 && info.Monitor.Top == 0 {
			return info, true
		}
	}

	if len(all) > 0 {
		return all[0], true
	}

	return monitorInfoEx{}, false
}

// centerWindow moves the hidden window to the middle of the chosen
// monitor's work area, a little above the middle so it reads as an
// overlay, as centerOnScreen does on macOS.
func centerWindow(mode string, display uint32) {
	window := launcherWindow()
	if window == 0 {
		return
	}

	info, ok := chosenMonitor(mode, display)
	if !ok {
		return
	}

	var frame rect
	_, _, _ = procGetWindowRect.Call(
		uintptr(window), uintptr(unsafe.Pointer(&frame)),
	)

	width := frame.Right - frame.Left
	height := frame.Bottom - frame.Top
	workWidth := info.Work.Right - info.Work.Left
	workHeight := info.Work.Bottom - info.Work.Top

	x := info.Work.Left + (workWidth-width)/2
	y := info.Work.Top + (workHeight-height)/2 - workHeight*8/100

	_, _, _ = procSetWindowPos.Call(
		uintptr(window), 0, uintptr(x), uintptr(y), 0, 0,
		swpNoSize|swpNoZOrder|swpNoActivate,
	)
}

// currentDisplay is the monitor the launcher is on now, 0 for none.
func currentDisplay() uint32 {
	window := launcherWindow()
	if window == 0 {
		return 0
	}

	monitor, _, _ := procMonitorFromWindow.Call(
		uintptr(window), monitorDefaultToNearest,
	)

	info, ok := monitorInfo(monitor)
	if !ok {
		return 0
	}

	return displayID(info)
}

// activeDisplays lists the attached monitors, so screenChoice can tell
// whether the remembered one is still there.
func activeDisplays() []uint32 {
	all := monitors()
	ids := make([]uint32, 0, len(all))

	for _, info := range all {
		ids = append(ids, displayID(info))
	}

	return ids
}

// handleReopen has nothing to hook: Wails' single-instance lock answers
// a second launch on Windows, so the show function is never needed here.
// There is no onReopen variable either, as on macOS: nothing would read it.
func handleReopen(show func()) {}

// activateApp brings the shown window to the front with the keyboard.
// It runs on the native thread, the one that registered the hotkey and
// so holds the right to take the foreground after a press. If Windows
// still refuses (the panel opened from the tray, say), the thread of
// the window in front lends its input queue to the thread that owns
// the launcher for the one call, the documented way round the
// foreground lock. The launcher belongs to Wails' thread, not to this
// one, so this thread joins that queue too: SetFocus only lands on a
// window whose input queue the calling thread shares.
func activateApp() {
	runNative(func() {
		window := launcherWindow()
		if window == 0 {
			return
		}

		ok, _, _ := procSetForegroundWindow.Call(uintptr(window))
		if ok != 0 {
			return
		}

		front, _, _ := procGetForegroundWindow.Call()
		frontThread, _, _ := procGetWindowThreadProcessId.Call(front, 0)
		windowThread, _, _ := procGetWindowThreadProcessId.Call(
			uintptr(window), 0,
		)
		ourThread := uintptr(windows.GetCurrentThreadId())

		if frontThread == 0 || windowThread == 0 {
			return
		}

		attachInput(frontThread, windowThread, true)
		attachInput(ourThread, windowThread, true)

		_, _, _ = procBringWindowToTop.Call(uintptr(window))
		_, _, _ = procSetForegroundWindow.Call(uintptr(window))
		_, _, _ = procSetFocus.Call(uintptr(window))

		attachInput(ourThread, windowThread, false)
		attachInput(frontThread, windowThread, false)
	})
}

// attachInput joins the input queue of one thread to the one of
// another, or parts them. AttachThreadInput refuses a thread joined to
// itself, so that case is skipped rather than left to fail.
func attachInput(from, to uintptr, attach bool) {
	if from == to {
		return
	}

	// The third argument is the Win32 BOOL: 1 joins, 0 parts.
	flag := uintptr(0)
	if attach {
		flag = 1
	}

	_, _, _ = procAttachThreadInput.Call(from, to, flag)
}
