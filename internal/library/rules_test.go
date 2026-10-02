package library

import (
	"runtime"
	"testing"
)

// Review Focus 5: on Windows the root check ignores case and accepts
// both slashes, and the extension is .exe or .lnk.
func TestWindowsAppPathRule(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows rules")
	}

	t.Setenv("ProgramFiles", `C:\Program Files`)
	home := `C:\Users\someone`

	for _, path := range []string{
		`C:\Program Files\Foo\foo.exe`,
		`C:\PROGRAM FILES\Foo\foo.exe`,
		`C:\Users\someone\AppData\Local\Programs\Foo\foo.lnk`,
	} {
		if detail := checkAppPath(path, home); detail != "" {
			t.Errorf("%s: %s", path, detail)
		}
	}

	for _, path := range []string{
		`C:\Users\someone\Downloads\foo.exe`,
		`C:\Program Files\Foo\foo.bat`,
		`Program Files\Foo\foo.exe`,
	} {
		if detail := checkAppPath(path, home); detail == "" {
			t.Errorf("%s: accepted", path)
		}
	}
}

// The defaults of Default() come from the platform.
func TestDefaultHotkeysAreThePlatformOnes(t *testing.T) {
	if Default().Settings.HotkeyApps != rules.defaultAppsHotkey {
		t.Error("Default ignores the platform rules")
	}
}
