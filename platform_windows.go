//go:build windows

package main

import (
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/drolosoft/hopto/internal/library"
)

// What newApp and the file picker need from Windows.

// keyboardSettingsURL opens Settings on the keyboard page.
const keyboardSettingsURL = "ms-settings:keyboard"

// symbolicHotkeysFile has no Windows counterpart: it stays empty so
// newApp builds the same field on both systems, and finderConflict
// never reads it here.
const symbolicHotkeysFile = ""

// startMenuPrograms is the Programs folder under either Start Menu.
const startMenuPrograms = `Microsoft\Windows\Start Menu\Programs`

// appRoots are the two Start Menus, all users first; System32 last for
// the stock tools, found only by typing.
func appRoots(home string) []string {
	return []string{
		filepath.Join(os.Getenv("ProgramData"), startMenuPrograms),
		filepath.Join(home, `AppData\Roaming`, startMenuPrograms),
		systemAppsRoot(),
	}
}

// systemAppsRoot is System32: discover lists a short fixed set of its
// tools (startmenu_windows.go).
func systemAppsRoot() string {
	return filepath.Join(os.Getenv("SystemRoot"), "System32")
}

// edgeAppsDir is the user's Start Menu: Edge puts its web apps there as
// shortcuts to msedge_proxy.exe.
func edgeAppsDir(home string) string {
	return filepath.Join(home, `AppData\Roaming`, startMenuPrograms)
}

// pickFolder is where the open dialog starts: the user's Start Menu.
func pickFolder() string {
	home, _ := os.UserHomeDir()

	return edgeAppsDir(home)
}

// pickFilters limits the dialog to programs and shortcuts.
func pickFilters() []runtime.FileFilter {
	return []runtime.FileFilter{
		{DisplayName: "Programs", Pattern: "*.exe;*.lnk"},
	}
}

// newLoginAgent is the Run key switch of login_windows.go.
func newLoginAgent(home string) loginAgent {
	return loginAgent{executable: os.Executable}
}

// finderConflict is a macOS question; Windows has no Finder.
func finderConflict(a *App, settings library.Settings) bool {
	return false
}
