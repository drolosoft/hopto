//go:build windows

package native

import (
	"log"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

// The native side of hopto on Windows: one goroutine locked to its OS
// thread owns a hidden window, the tray icon, the two global hotkeys and
// the one message loop their events arrive in. Nothing here is called
// from Wails' thread; the engine's goroutines hand work over with
// runNative, which posts a message to that thread. The tray icon and its
// menu are in tray_windows.go, the hotkeys in hotkey_windows.go.

// The Win32 entry points, loaded once from the system folder only
// (NewLazySystemDLL never looks in the working directory).
var (
	user32  = windows.NewLazySystemDLL("user32.dll")
	shell32 = windows.NewLazySystemDLL("shell32.dll")

	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procRegisterHotKey      = user32.NewProc("RegisterHotKey")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procLoadIconW           = user32.NewProc("LoadIconW")
	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")

	procRegisterWindowMessageW = user32.NewProc("RegisterWindowMessageW")
)

// Window messages (winuser.h). wmSetFocus is the one a window gets when
// it has just gained the keyboard, which nudgeFocus posts to Wails'
// window. trayMessage is our own number in the WM_APP range for the
// icon's clicks; nativeJob is the one runNative posts to run a function
// on the native thread.
const (
	wmNull      = 0x0000
	wmDestroy   = 0x0002
	wmSetFocus  = 0x0007
	wmHotkey    = 0x0312
	wmLButtonUp = 0x0202
	wmRButtonUp = 0x0205
	wmApp       = 0x8000
	trayMessage = wmApp + 1
	nativeJob   = wmApp + 2
)

// Shell_NotifyIcon actions and flags (shellapi.h).
const (
	nimAdd     = 0x0
	nimDelete  = 0x2
	nifMessage = 0x1
	nifIcon    = 0x2
	nifTip     = 0x4
)

// Menu flags (winuser.h): a text entry, a rule, a tick; and the popup
// options: return the chosen id instead of sending WM_COMMAND, take the
// right button too, and send no WM_ENTERMENULOOP noise.
const (
	mfString       = 0x0000
	mfSeparator    = 0x0800
	mfChecked      = 0x0008
	tpmRightButton = 0x0002
	tpmNoNotify    = 0x0080
	tpmReturnCmd   = 0x0100
)

// modNoRepeat makes a held key fire once (winuser.h MOD_NOREPEAT).
const modNoRepeat = 0x4000

// trayClassName is the class of the hidden tray window. trayIconID
// tells the shell which of the window's icons a call means: hopto has
// one icon, so one id, and adding and deleting it must agree on it.
const (
	trayClassName = "hoptoTray"
	trayIconID    = 1
)

// appIconResource is the id Wails gives the icon it embeds from
// build/windows/icon.ico (winc.AppIconID); idiApplication is the stock
// icon LoadIcon falls back to.
const (
	appIconResource = 3
	idiApplication  = 32512
)

// wndClassEx is WNDCLASSEXW.
type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   windows.Handle
	Icon       windows.Handle
	Cursor     windows.Handle
	Background windows.Handle
	MenuName   *uint16
	ClassName  *uint16
	IconSm     windows.Handle
}

// point is POINT.
type point struct {
	X, Y int32
}

// message is MSG.
type message struct {
	Window  windows.Handle
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Point   point
}

// notifyIconData is NOTIFYICONDATAW, the Vista layout (976 bytes on a
// 64-bit build): the fields hopto does not set stay zero.
type notifyIconData struct {
	Size            uint32
	Window          windows.Handle
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            windows.Handle
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUID            windows.GUID
	BalloonIcon     windows.Handle
}

// thread is the state the native thread owns. items is read under mu
// because Install and SetChecked write it from the engine's goroutines;
// the menu itself is built from items at every click, so no menu handle
// outlives a click and the ticks are always the latest.
var thread struct {
	once   sync.Once
	ready  chan struct{}
	window windows.Handle

	mu    sync.Mutex
	items []MenuItem
	jobs  []func()
}

// startNative starts the native thread once; every entry point calls it,
// so whichever the engine reaches first brings the thread up.
func startNative() {
	thread.once.Do(func() {
		thread.ready = make(chan struct{})
		go nativeLoop()
	})

	<-thread.ready
}

// nativeLoop is the whole life of the native thread: the window, the
// tray icon, and then messages until the process exits. Nothing destroys
// the window, so the icon is removed by Shutdown instead.
func nativeLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	window, err := createTrayWindow()
	if err != nil {
		log.Printf("native: tray window: %v", err)
		close(thread.ready)
		return
	}

	thread.window = window
	registerTaskbarCreated()
	addTrayIcon(window)
	close(thread.ready)

	var msg message
	for {
		result, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(result) <= 0 {
			break
		}

		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// createTrayWindow registers the class and creates a top-level window
// that is never shown. A message-only window (HWND_MESSAGE) would do for
// WM_HOTKEY, but the shell posts the tray icon's clicks to a window it
// can find, and a message-only window never receives broadcasts such as
// TaskbarCreated, so this is an ordinary hidden one, as every tray
// library makes it.
func createTrayWindow() (windows.Handle, error) {
	instance, err := moduleHandle()
	if err != nil {
		return 0, err
	}

	className, err := windows.UTF16PtrFromString(trayClassName)
	if err != nil {
		return 0, err
	}

	class := wndClassEx{
		WndProc:   windows.NewCallback(trayWindowProc),
		Instance:  instance,
		ClassName: className,
	}
	class.Size = uint32(unsafe.Sizeof(class))

	atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class)))
	if atom == 0 {
		return 0, err
	}

	window, _, err := procCreateWindowExW.Call(
		0, uintptr(unsafe.Pointer(className)), 0, 0,
		0, 0, 0, 0, 0, 0, uintptr(instance), 0,
	)
	if window == 0 {
		return 0, err
	}

	return windows.Handle(window), nil
}

// trayWindowProc is where every native event lands: a hotkey, a click
// on the icon, or a job posted by runNative. Each one only starts a
// goroutine towards the engine, as the Carbon handler does on macOS.
func trayWindowProc(
	window windows.Handle, msg uint32, wParam, lParam uintptr,
) uintptr {
	// Explorer started again and the icon it drew is gone. The number
	// is not a constant, so it cannot be a case of the switch below.
	if taskbarCreated != 0 && msg == taskbarCreated {
		readdTrayIcon(window)

		return 0
	}

	switch msg {
	case wmHotkey:
		hotkeyPressed(uint32(wParam))

		return 0

	case trayMessage:
		mouse := loWord(lParam)
		if mouse == wmLButtonUp || mouse == wmRButtonUp {
			showTrayMenu(window)
		}

		return 0

	case nativeJob:
		runPendingJobs()

		return 0

	case wmDestroy:
		removeTrayIcon(window)

		return 0
	}

	result, _, _ := procDefWindowProcW.Call(
		uintptr(window), uintptr(msg), wParam, lParam,
	)

	return result
}

// loWord is the low 16 bits of a message parameter (LOWORD), where the
// shell puts the mouse message of a click on the tray icon.
func loWord(value uintptr) uint32 {
	return uint32(value & 0xFFFF)
}

// runNative runs fn on the native thread and waits for it. It is what
// every engine-side entry point uses, so user32 only ever sees the
// thread that owns the window.
func runNative(fn func()) {
	startNative()

	if thread.window == 0 {
		return
	}

	done := make(chan struct{})
	thread.mu.Lock()
	thread.jobs = append(thread.jobs, func() {
		fn()
		close(done)
	})
	thread.mu.Unlock()

	posted, _, _ := procPostMessageW.Call(
		uintptr(thread.window), nativeJob, 0, 0,
	)
	if posted == 0 {
		// Nobody will run the job, so waiting for it would block for good.
		return
	}

	<-done
}

// runPendingJobs drains the queue on the native thread.
func runPendingJobs() {
	thread.mu.Lock()
	jobs := thread.jobs
	thread.jobs = nil
	thread.mu.Unlock()

	for _, job := range jobs {
		job()
	}
}

// moduleHandle is the handle of the running executable, which is where
// the window class and the embedded icon belong. x/sys has no
// GetModuleHandle, but the Ex variant with no name and no flags is the
// same call.
func moduleHandle() (windows.Handle, error) {
	var instance windows.Handle
	err := windows.GetModuleHandleEx(0, nil, &instance)

	return instance, err
}
