//go:build windows

package main

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
// runNative, which posts a message to that thread.

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
)

// Window messages (winuser.h). trayMessage is our own number in the
// WM_APP range for the icon's clicks; nativeJob is the one runNative
// posts to run a function on the native thread.
const (
	wmNull      = 0x0000
	wmDestroy   = 0x0002
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

// native is the state the native thread owns. items is read under mu
// because Install and SetChecked write it from the engine's goroutines;
// the menu itself is built from items at every click, so no menu handle
// outlives a click and the ticks are always the latest.
var native struct {
	once   sync.Once
	ready  chan struct{}
	window windows.Handle

	mu    sync.Mutex
	items []menuItem
	jobs  []func()
}

// trayClassName is the class of the hidden tray window.
const trayClassName = "hoptoTray"

// startNative starts the native thread once; every entry point calls it,
// so whichever the engine reaches first brings the thread up.
func startNative() {
	native.once.Do(func() {
		native.ready = make(chan struct{})
		go nativeLoop()
	})

	<-native.ready
}

// nativeLoop is the whole life of the native thread: the window, the
// tray icon, and then messages until the window is destroyed.
func nativeLoop() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	window, err := createTrayWindow()
	if err != nil {
		log.Printf("native: tray window: %v", err)
		native.window = 0
		close(native.ready)
		return
	}

	native.window = window
	addTrayIcon(window)
	close(native.ready)

	var msg message
	for {
		got, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&msg)), 0, 0, 0,
		)
		if int32(got) <= 0 {
			break
		}

		_, _, _ = procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		_, _, _ = procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}
}

// createTrayWindow registers the class and creates a top-level window
// that is never shown. A message-only window (HWND_MESSAGE) would do for
// WM_HOTKEY, but the shell posts the tray icon's clicks to a window it
// can find, so this is an ordinary hidden one, as every tray library
// makes it.
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

	atom, _, err := procRegisterClassExW.Call(
		uintptr(unsafe.Pointer(&class)),
	)
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
	window windows.Handle,
	msg uint32,
	wParam, lParam uintptr,
) uintptr {
	switch msg {
	case wmHotkey:
		id := uint32(wParam)
		tab := tabForHotkey(id)
		log.Printf("hotkey %d pressed: %s", id, tab)
		go onHotkey(tab)

		return 0

	case trayMessage:
		low := uint32(lParam & 0xFFFF)
		if low == wmLButtonUp || low == wmRButtonUp {
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

// runNative runs fn on the native thread and waits for it. It is what
// every engine-side entry point uses, so user32 only ever sees the
// thread that owns the window.
func runNative(fn func()) {
	startNative()

	if native.window == 0 {
		return
	}

	done := make(chan struct{})
	native.mu.Lock()
	native.jobs = append(native.jobs, func() {
		fn()
		close(done)
	})
	native.mu.Unlock()

	_, _, _ = procPostMessageW.Call(uintptr(native.window), nativeJob, 0, 0)
	<-done
}

// runPendingJobs drains the queue on the native thread.
func runPendingJobs() {
	native.mu.Lock()
	jobs := native.jobs
	native.jobs = nil
	native.mu.Unlock()

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

// appIcon loads the icon Wails embedded, or the stock application icon
// on a build made without one.
func appIcon() windows.Handle {
	instance, _ := moduleHandle()

	icon, _, _ := procLoadIconW.Call(uintptr(instance), appIconResource)
	if icon == 0 {
		icon, _, _ = procLoadIconW.Call(0, idiApplication)
	}

	return windows.Handle(icon)
}

// addTrayIcon puts hopto in the notification area, with its clicks sent
// to the window as trayMessage.
func addTrayIcon(window windows.Handle) {
	data := notifyIconData{
		Window:          window,
		ID:              1,
		Flags:           nifMessage | nifIcon | nifTip,
		CallbackMessage: trayMessage,
		Icon:            appIcon(),
	}
	data.Size = uint32(unsafe.Sizeof(data))
	copy(data.Tip[:], windows.StringToUTF16("hopto"))

	ok, _, err := procShellNotifyIconW.Call(
		nimAdd, uintptr(unsafe.Pointer(&data)),
	)
	if ok == 0 {
		log.Printf("native: tray icon: %v", err)
		return
	}

	log.Printf("native: tray icon added")
}

// removeTrayIcon takes the icon away when the window goes.
func removeTrayIcon(window windows.Handle) {
	data := notifyIconData{Window: window, ID: 1}
	data.Size = uint32(unsafe.Sizeof(data))
	_, _, _ = procShellNotifyIconW.Call(
		nimDelete, uintptr(unsafe.Pointer(&data)),
	)
}

// showTrayMenu builds the menu from the current items, shows it at the
// pointer and hands the chosen tag to the engine. SetForegroundWindow
// before and a WM_NULL after are what TrackPopupMenu's documentation
// asks for, so the menu closes on a click outside it.
func showTrayMenu(window windows.Handle) {
	native.mu.Lock()
	items := append([]menuItem{}, native.items...)
	native.mu.Unlock()

	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer func() {
		_, _, _ = procDestroyMenu.Call(menu)
	}()

	for _, item := range items {
		if item.Tag == menuSeparator {
			_, _, _ = procAppendMenuW.Call(menu, mfSeparator, 0, 0)
			continue
		}

		flags := uintptr(mfString)
		if item.Checked {
			flags |= mfChecked
		}

		title, err := windows.UTF16PtrFromString(item.Title)
		if err != nil {
			continue
		}

		_, _, _ = procAppendMenuW.Call(
			menu, flags, uintptr(item.Tag),
			uintptr(unsafe.Pointer(title)),
		)
	}

	var cursor point
	_, _, _ = procGetCursorPos.Call(uintptr(unsafe.Pointer(&cursor)))
	_, _, _ = procSetForegroundWindow.Call(uintptr(window))

	chosen, _, _ := procTrackPopupMenu.Call(
		menu, tpmReturnCmd|tpmRightButton|tpmNoNotify,
		uintptr(cursor.X), uintptr(cursor.Y), 0, uintptr(window), 0,
	)
	_, _, _ = procPostMessageW.Call(uintptr(window), wmNull, 0, 0)

	if chosen == 0 {
		return
	}

	log.Printf("menu %d picked", chosen)
	go onMenu(int(chosen))
}

// registerHotkey registers one shortcut on the native thread and records
// what Windows said, so the welcome can show a taken combination. A held
// key fires once.
func registerHotkey(id uint32, hotkey Hotkey) {
	runNative(func() {
		ok, _, err := procRegisterHotKey.Call(
			uintptr(native.window), uintptr(id),
			uintptr(hotkey.Modifiers|modNoRepeat), uintptr(hotkey.KeyCode),
		)

		if ok == 0 {
			status := int32(0)
			if errno, isErrno := err.(windows.Errno); isErrno {
				status = int32(errno)
			}

			recordHotkeyStatus(id, status)
			log.Printf(
				"hotkey %d: RegisterHotKey failed with status %d",
				id, status,
			)
			return
		}

		recordHotkeyStatus(id, 0)
		log.Printf("hotkey %d registered", id)
	})
}
