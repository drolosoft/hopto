package discover

import (
	"path/filepath"
	"sort"
	"strings"
)

// edgeAppsDir, under the home folder, is where Edge (and Chrome) put the
// bundles of "installed" web apps. Every .app in it is a launcher entry,
// found on each showing so a newly installed app appears without a rebuild.
const edgeAppsDir = "Applications/Edge Apps.localized"

// EdgeAppsDir is the Edge web apps folder of a home directory.
func EdgeAppsDir(home string) string {
	return filepath.Join(home, edgeAppsDir)
}

// sortByName orders apps the way a list should read, ignoring case.
func sortByName(apps []App) {
	sort.SliceStable(apps, func(left, right int) bool {
		return strings.ToLower(apps[left].Name) < strings.ToLower(apps[right].Name)
	})
}
