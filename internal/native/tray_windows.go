//go:build windows

// The tray icon on Windows: adding it to the notification area, adding it
// again after Explorer restarts, taking it away on quit, and the menu a
// click on it shows. It all runs on the native thread.

package native

import (
	"log"
	"unsafe"

	"golang.org/x/sys/windows"
)

// taskbarCreated is the number Windows gave the "TaskbarCreated"
// message, which Explorer broadcasts to every top-level window when it
// starts again after a crash or a restart: the notification area comes
// back empty, and the tray is the only place to quit hopto, so the icon
// has to be added again. It is 0 until the native thread registers it,
// and only that thread reads or writes it, as with readdFailureLogged.
var taskbarCreated uint32

// readdFailureLogged keeps a failing re-add to one line in the log.
// Explorer also sends TaskbarCreated when the DPI changes, with the icon
// still there, and NIM_ADD of an id that exists fails harmlessly.
var readdFailureLogged bool

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

// registerTaskbarCreated asks Windows for the number of the message
// Explorer broadcasts when it starts. A failure leaves taskbarCreated at
// 0, which matches no message: hopto then works, minus the icon coming
// back after an Explorer restart.
func registerTaskbarCreated() {
	name, err := windows.UTF16PtrFromString("TaskbarCreated")
	if err != nil {
		return
	}

	number, _, err := procRegisterWindowMessageW.Call(
		uintptr(unsafe.Pointer(name)),
	)
	if number == 0 {
		log.Printf("native: TaskbarCreated: %v", err)
		return
	}

	taskbarCreated = uint32(number)
}

// notifyAdd asks the shell for hopto's icon in the notification area,
// with its clicks sent to the window as trayMessage.
func notifyAdd(window windows.Handle) error {
	data := notifyIconData{
		Window:          window,
		ID:              trayIconID,
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
		return err
	}

	return nil
}

// addTrayIcon puts hopto in the notification area at start-up.
func addTrayIcon(window windows.Handle) {
	if err := notifyAdd(window); err != nil {
		log.Printf("native: tray icon: %v", err)
		return
	}

	log.Printf("native: tray icon added")
}

// readdTrayIcon puts the icon back after Explorer restarted. A failure
// is logged once only: the same message also arrives while the icon is
// still there, and the add then fails through no fault of hopto's.
func readdTrayIcon(window windows.Handle) {
	err := notifyAdd(window)
	if err == nil {
		log.Printf("native: tray icon added again")
		return
	}

	if !readdFailureLogged {
		readdFailureLogged = true
		log.Printf("native: tray icon again: %v", err)
	}
}

// removeTrayIcon takes the icon away when the window goes.
func removeTrayIcon(window windows.Handle) {
	data := notifyIconData{Window: window, ID: trayIconID}
	data.Size = uint32(unsafe.Sizeof(data))
	_, _, _ = procShellNotifyIconW.Call(
		nimDelete, uintptr(unsafe.Pointer(&data)),
	)
}

// Shutdown takes the tray icon away before the process exits:
// Wails ends with a WM_QUIT to its own thread, so the native window is
// never destroyed and its WM_DESTROY never arrives. Nothing to do when
// the native thread never started.
func Shutdown() {
	if thread.window == 0 {
		return
	}

	runNative(func() {
		removeTrayIcon(thread.window)
	})
}

// showTrayMenu builds the menu from the current items, shows it at the
// pointer and hands the chosen tag to the engine. SetForegroundWindow
// before and a WM_NULL after are what TrackPopupMenu's documentation
// asks for, so the menu closes on a click outside it.
func showTrayMenu(window windows.Handle) {
	thread.mu.Lock()
	items := append([]MenuItem{}, thread.items...)
	thread.mu.Unlock()

	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		return
	}
	defer func() {
		_, _, _ = procDestroyMenu.Call(menu)
	}()

	for _, item := range items {
		if item.Separator {
			_, _, _ = procAppendMenuW.Call(menu, mfSeparator, 0, 0)
			continue
		}

		flags := uintptr(mfString)
		if item.Checked {
			flags |= mfChecked
		}

		title, err := windows.UTF16PtrFromString(item.Label)
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

	menuPicked(int(chosen))
}
