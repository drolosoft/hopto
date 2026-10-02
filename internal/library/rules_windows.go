//go:build windows

package library

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// bundlePattern keeps bundle_id a well-formed field on Windows too; the
// adapter of `open` never uses it here.
var bundlePattern = regexp.MustCompile(`^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`)

// windowsAppRoots are where Windows installs programs and keeps their
// Start Menu shortcuts: the two Program Files, the per-user Programs,
// the two Start Menus and System32. A root whose environment variable
// is empty is skipped: joining "" with a relative tail would give a
// relative path that matches nothing sensible.
func windowsAppRoots(home string) []string {
	var roots []string

	for _, name := range []string{
		"ProgramFiles", "ProgramFiles(x86)", "ProgramW6432",
	} {
		if value := os.Getenv(name); value != "" {
			roots = append(roots, value)
		}
	}

	programData := os.Getenv("ProgramData")
	if programData != "" {
		roots = append(roots, filepath.Join(
			programData, `Microsoft\Windows\Start Menu\Programs`,
		))
	}

	roots = append(roots,
		filepath.Join(home, `AppData\Local\Programs`),
		filepath.Join(
			home, `AppData\Roaming\Microsoft\Windows\Start Menu\Programs`,
		),
	)

	systemRoot := os.Getenv("SystemRoot")
	if systemRoot != "" {
		roots = append(roots, filepath.Join(systemRoot, "System32"))
	}

	return roots
}

// rules on Windows: an .exe or a .lnk under the program folders, compared
// without case and with either slash; a browser is the path of an exe.
var rules = platformRules{
	appExtensions: []string{".exe", ".lnk"},
	appRoots:      windowsAppRoots,
	appPathHint: "path must be under Program Files, the Start Menu " +
		"or AppData\\Local\\Programs",
	underRoot: func(path, root string) bool {
		clean := strings.ToLower(filepath.Clean(path))
		base := strings.ToLower(filepath.Clean(root))

		return strings.HasPrefix(clean, base+string(filepath.Separator))
	},
	browserOK: func(value string) bool {
		return filepath.IsAbs(value) &&
			strings.EqualFold(filepath.Ext(value), ".exe")
	},
	browserHint:        "secondary_browser must be the path of a browser's exe",
	defaultAppsHotkey:  "ctrl+shift+space",
	defaultLinksHotkey: "ctrl+alt+space",
}
