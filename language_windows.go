//go:build windows

package main

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")

	procGetUserDefaultLocaleName = kernel32.NewProc(
		"GetUserDefaultLocaleName",
	)
)

// localeNameMaxLength is LOCALE_NAME_MAX_LENGTH (winnls.h).
const localeNameMaxLength = 85

// systemLanguage reads the user's locale name ("es-ES") and reduces it
// to a language hopto speaks.
func systemLanguage() string {
	var name [localeNameMaxLength]uint16

	length, _, _ := procGetUserDefaultLocaleName.Call(
		uintptr(unsafe.Pointer(&name[0])), localeNameMaxLength,
	)
	if length == 0 {
		return languageEnglish
	}

	tag := strings.ToLower(windows.UTF16ToString(name[:]))

	return languageFor("auto", tag)
}
