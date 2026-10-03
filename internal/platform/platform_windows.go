//go:build windows

package platform

import (
	"os"
	"path/filepath"

	"github.com/drolosoft/hopto/internal/library"
)

// What newApp and the file picker need from Windows.

// KeyboardSettingsURL opens Settings on the keyboard page.
const KeyboardSettingsURL = "ms-settings:keyboard"

// SymbolicHotkeysFile has no Windows counterpart: it stays empty so
// newApp builds the same field on both systems, and FinderConflict
// never reads it here.
const SymbolicHotkeysFile = ""

// startMenuPrograms is the Programs folder under either Start Menu.
const startMenuPrograms = `Microsoft\Windows\Start Menu\Programs`

// AppRoots are the two Start Menus, all users first; System32 last for
// the stock tools, found only by typing.
func AppRoots(home string) []string {
	return []string{
		filepath.Join(os.Getenv("ProgramData"), startMenuPrograms),
		filepath.Join(home, `AppData\Roaming`, startMenuPrograms),
		SystemAppsRoot(),
	}
}

// SystemAppsRoot is System32: discover lists a short fixed set of its
// tools (systemTools in internal/discover).
func SystemAppsRoot() string {
	return filepath.Join(os.Getenv("SystemRoot"), "System32")
}

// EdgeAppsDir is the user's Start Menu: Edge puts its web apps there as
// shortcuts to msedge_proxy.exe.
func EdgeAppsDir(home string) string {
	return filepath.Join(home, `AppData\Roaming`, startMenuPrograms)
}

// PickFolder is where the open dialog starts: the user's Start Menu.
func PickFolder() string {
	home, _ := os.UserHomeDir()

	return EdgeAppsDir(home)
}

// PickFilters limits the dialog to programs and shortcuts.
func PickFilters() []FileFilter {
	return []FileFilter{
		{Name: "Programs", Pattern: "*.exe;*.lnk"},
	}
}

// NewLoginAgent is the Run key switch of login_windows.go. It takes the
// home folder only to share the macOS signature: the Run key is per user
// already.
func NewLoginAgent(_ string) LoginAgent {
	return runValueLogin{executable: os.Executable}
}

// FinderConflict is a macOS question; Windows has no Finder, and the
// parameters are there only to share the macOS signature.
func FinderConflict(_ string, _ library.Settings) bool {
	return false
}
