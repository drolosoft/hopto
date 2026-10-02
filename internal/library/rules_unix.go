//go:build !windows

package library

import (
	"path/filepath"
	"regexp"
	"strings"
)

// bundlePattern is a reverse-DNS bundle identifier, as `open -b` wants it.
var bundlePattern = regexp.MustCompile(`^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`)

// rules on macOS: a .app under the folders macOS installs apps in, a
// bundle id for the browser, Command-based shortcuts.
var rules = platformRules{
	appExtensions: []string{".app"},
	appRoots: func(home string) []string {
		return []string{
			"/Applications",
			"/System/Applications",
			filepath.Join(home, "Applications"),
		}
	},
	appPathHint: "path must be under /Applications, /System/Applications " +
		"or ~/Applications",
	underRoot: func(path, root string) bool {
		return strings.HasPrefix(path, root+"/")
	},
	browserOK: bundlePattern.MatchString,
	browserHint: "secondary_browser must be a bundle id like " +
		"com.apple.Safari",
	defaultAppsHotkey:  "cmd+shift+space",
	defaultLinksHotkey: "cmd+option+space",
}
