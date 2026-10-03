//go:build windows

package platform

import (
	"errors"
	"fmt"
	"log"
	"unsafe"

	"golang.org/x/sys/windows"
)

// shell32 is the library ShellExecute lives in. The lazy handle is cheap;
// each file loads the DLL it uses.
var shell32 = windows.NewLazySystemDLL("shell32.dll")

// procShellExecuteW is shell32's ShellExecuteW, which starts a file, a
// URL or a program the way a double click would.
var procShellExecuteW = shell32.NewProc("ShellExecuteW")

// swShowNormal is SW_SHOWNORMAL (winuser.h): 1, show the window at its
// normal size and position.
const swShowNormal = 1

// shellExecuteFailure is the largest return value that means failure:
// ShellExecute answers an error code up to 32, and an instance handle
// above it.
const shellExecuteFailure = 32

// errShellExecute: ShellExecute returned one of its error codes.
var errShellExecute = errors.New("ShellExecute failed")

// RunOpen is the Windows side of /usr/bin/open: ShellExecute, the only
// way hopto starts anything, so a .lnk, an .exe and a URL all open the
// way a double click opens them.
func RunOpen(args ...string) error {
	verb, file, params, err := openArguments(args)
	if err != nil {
		return err
	}

	verbPointer, err := windows.UTF16PtrFromString(verb)
	if err != nil {
		return err
	}

	filePointer, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}

	// ShellExecute takes NULL when there are no parameters.
	var paramsPointer *uint16
	if params != "" {
		paramsPointer, err = windows.UTF16PtrFromString(params)
		if err != nil {
			return err
		}
	}

	result, _, _ := procShellExecuteW.Call(
		0,
		uintptr(unsafe.Pointer(verbPointer)),
		uintptr(unsafe.Pointer(filePointer)),
		uintptr(unsafe.Pointer(paramsPointer)),
		0,
		swShowNormal,
	)
	if result <= shellExecuteFailure {
		log.Printf("open %v: ShellExecute code %d", args, result)

		return fmt.Errorf("%w: code %d", errShellExecute, result)
	}

	return nil
}
