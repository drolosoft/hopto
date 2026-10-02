//go:build !windows

package main

import (
	"os"
	"path/filepath"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/drolosoft/hopto/internal/discover"
	"github.com/drolosoft/hopto/internal/library"
)

// What newApp and the file picker need from macOS; platform_windows.go
// answers the same questions for Windows.

// keyboardSettingsURL opens System Settings on the Keyboard pane, where
// Keyboard Shortcuts → Spotlight holds the Finder's ⌘⌥Space.
const keyboardSettingsURL = "x-apple.systempreferences:" +
	"com.apple.Keyboard-Settings.extension"

// appRoots are the folders discovery scans, the system one last (its
// apps are only found by typing).
func appRoots(home string) []string {
	return []string{
		"/Applications",
		filepath.Join(home, "Applications"),
		systemAppsRoot(),
	}
}

// systemAppsRoot holds the apps macOS ships (Mail, Notes, Terminal).
func systemAppsRoot() string {
	return "/System/Applications"
}

// edgeAppsDir is where Edge keeps its web app bundles.
func edgeAppsDir(home string) string {
	return discover.EdgeAppsDir(home)
}

// pickFolder is where the open panel starts: where apps are installed.
func pickFolder() string {
	return "/Applications"
}

// pickFilters limits the open panel to .app bundles.
func pickFilters() []runtime.FileFilter {
	return []runtime.FileFilter{
		{DisplayName: "Applications", Pattern: "*.app"},
	}
}

// newLoginAgent is the LaunchAgent switch of login_unix.go.
func newLoginAgent(home string) loginAgent {
	return loginAgent{path: launchAgentPath(home), executable: os.Executable}
}

// finderConflict says whether a shortcut of the settings is ⌘⌥Space
// while the Finder still keeps it.
func finderConflict(a *App, settings library.Settings) bool {
	return usesFinderShortcut(settings) &&
		finderSearchShortcutEnabled(a.symbolicHotkeys)
}
