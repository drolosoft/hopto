//go:build !windows

package platform

import (
	"os"
	"path/filepath"

	"github.com/drolosoft/hopto/internal/discover"
	"github.com/drolosoft/hopto/internal/library"
)

// What newApp and the file picker need from macOS; platform_windows.go
// answers the same questions for Windows.

// KeyboardSettingsURL opens System Settings on the Keyboard pane, where
// Keyboard Shortcuts → Spotlight holds the Finder's ⌘⌥Space.
const KeyboardSettingsURL = "x-apple.systempreferences:" +
	"com.apple.Keyboard-Settings.extension"

// AppRoots are the folders discovery scans, the system one last (its
// apps are only found by typing).
func AppRoots(home string) []string {
	return []string{
		"/Applications",
		filepath.Join(home, "Applications"),
		SystemAppsRoot(),
	}
}

// SystemAppsRoot holds the apps macOS ships (Mail, Notes, Terminal).
func SystemAppsRoot() string {
	return "/System/Applications"
}

// EdgeAppsDir is where Edge keeps its web app bundles.
func EdgeAppsDir(home string) string {
	return discover.EdgeAppsDir(home)
}

// PickFolder is where the open panel starts: where apps are installed.
func PickFolder() string {
	return "/Applications"
}

// PickFilters limits the open panel to .app bundles.
func PickFilters() []FileFilter {
	return []FileFilter{
		{Name: "Applications", Pattern: "*.app"},
	}
}

// NewLoginAgent is the LaunchAgent switch of login_unix.go.
func NewLoginAgent(home string) LoginAgent {
	return launchdLogin{
		path:       launchAgentPath(home),
		executable: os.Executable,
	}
}

// FinderConflict says whether a shortcut of the settings is ⌘⌥Space
// while the Finder still keeps it. symbolicHotkeysFile is the full path of
// the file the App keeps, so this package need not know the App.
func FinderConflict(
	symbolicHotkeysFile string, settings library.Settings,
) bool {
	return usesFinderShortcut(settings) &&
		finderSearchShortcutEnabled(symbolicHotkeysFile)
}
