//go:build windows

package platform

import (
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// kernel32 is the library GetUserDefaultLocaleName lives in.
var kernel32 = windows.NewLazySystemDLL("kernel32.dll")

// procGetUserDefaultLocaleName is kernel32's GetUserDefaultLocaleName,
// which writes the user's locale name, like "es-ES", into a buffer.
var procGetUserDefaultLocaleName = kernel32.NewProc(
	"GetUserDefaultLocaleName",
)

// localeNameMaxLength is LOCALE_NAME_MAX_LENGTH (winnls.h).
const localeNameMaxLength = 85

// SystemLanguage reads the user's locale name ("es-ES") and reduces it
// to a language hopto speaks.
func SystemLanguage() string {
	var name [localeNameMaxLength]uint16

	length, _, _ := procGetUserDefaultLocaleName.Call(
		uintptr(unsafe.Pointer(&name[0])), localeNameMaxLength,
	)
	if length == 0 {
		return LanguageEnglish
	}

	tag := strings.ToLower(windows.UTF16ToString(name[:]))

	return LanguageFor("auto", tag)
}
