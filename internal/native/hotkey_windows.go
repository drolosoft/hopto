//go:build windows

package native

import (
	"hash/fnv"
	"log"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/drolosoft/hopto/internal/library"
	"github.com/drolosoft/hopto/internal/platform"
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

// launcherWindow is Wails' window, found by the class name main.go set.
func launcherWindow() windows.Handle {
	className, _ := windows.UTF16PtrFromString(platform.WindowClassName)
	window, _, _ := procFindWindowW.Call(
		uintptr(unsafe.Pointer(className)), 0,
	)

	return windows.Handle(window)
}

// BecomeAccessory keeps the launcher off the taskbar and out of
// Alt+Tab: the Windows side of the accessory activation policy. Done
// while the window is still hidden, so the style change never flickers.
func BecomeAccessory() {
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

// RegisterHotkeys binds the apps shortcut to the apps tab and the links
// shortcut to the links tab, on the native thread; a press reaches
// hooks.Toggle.
func RegisterHotkeys(apps, links platform.Hotkey) {
	registerHotkey(platform.HotkeyApps, apps)
	registerHotkey(platform.HotkeyLinks, links)
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

// Shutdown takes the tray icon away before the process exits:
// Wails ends with a WM_QUIT to its own thread, so the native window is
// never destroyed and its WM_DESTROY never arrives. Nothing to do when
// the native thread never started.
func Shutdown() {
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
	if mode == library.ScreenMouse {
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

	if mode == library.ScreenLast {
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

// CenterWindow moves the hidden window to the middle of the chosen
// monitor's work area, a little above the middle so it reads as an
// overlay, as centerOnScreen does on macOS.
func CenterWindow(mode string, display uint32) {
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

// CurrentDisplay is the monitor the launcher is on now, 0 for none.
func CurrentDisplay() uint32 {
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

// ActiveDisplays lists the attached monitors, so screenChoice can tell
// whether the remembered one is still there.
func ActiveDisplays() []uint32 {
	all := monitors()
	ids := make([]uint32, 0, len(all))

	for _, info := range all {
		ids = append(ids, displayID(info))
	}

	return ids
}

// HandleReopen has nothing to hook: Wails' single-instance lock answers
// a second launch on Windows, so hooks.Reopen is never called from here.
func HandleReopen() {}

// Activate brings the window to the front with the keyboard once
// Wails has queued the show (WindowShow posts the work to Wails' thread
// and returns at once). It runs on the native thread, the one that
// registered the hotkey and so holds the right to take the foreground
// after a press. If Windows still refuses (the panel opened from the
// tray, say), the thread of the window in front lends its input queue
// to the thread that owns the launcher for the one call, the documented
// way round the foreground lock. The keyboard itself is handed to the
// page by Wails, nudged by nudgeFocus.
func Activate() {
	runNative(func() {
		window := launcherWindow()
		if window == 0 {
			return
		}

		bringForward(window)
		nudgeFocus(window)
	})
}

// bringForward makes the launcher the foreground window: a plain
// SetForegroundWindow first, then the input-queue loan described on
// Activate when Windows refuses.
func bringForward(window windows.Handle) {
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
	ok, _, _ = procSetForegroundWindow.Call(uintptr(window))

	attachInput(ourThread, windowThread, false)
	attachInput(frontThread, windowThread, false)

	// Left in the log so a panel that opens behind another window on
	// some machine can be told from one that never opened.
	if ok == 0 {
		log.Println("native: foreground refused")
	}
}

// wmSetFocus is the message a window gets when it has just gained the
// keyboard focus (winuser.h WM_SETFOCUS).
const wmSetFocus = 0x0007

// nudgeFocus asks Wails to hand the keyboard to the WebView2 widget.
// Wails does that in its WM_SETFOCUS handler, with Chromium's own
// MoveFocus on its own thread, and the activation bringForward starts
// ends in that message. The message never comes when the launcher was
// already the active window while hidden (right after start, or when
// nothing else took the keyboard after a hide): the focus then stays on
// the bare window and the page sees no key. Posting the same message
// runs the handler in both cases; a second MoveFocus after a real
// activation changes nothing. Moving the focus from this thread
// instead, with SetFocus under an input-queue loan, raced the pending
// activation and now and then left the desktop with no foreground
// window at all.
//
// The message is posted, never sent, and after the show: both land in
// the queue of Wails' thread in that order, so the handler always runs
// on a visible window. That order matters, because go-webview2 exits
// the process on a MoveFocus error rather than returning it.
func nudgeFocus(window windows.Handle) {
	posted, _, _ := procPostMessageW.Call(
		uintptr(window), wmSetFocus, 0, 0,
	)
	if posted == 0 {
		log.Println("native: focus: nudge not posted")
	}
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
