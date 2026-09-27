package discover

import (
	"os"
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

// ScanEdgeApps lists the web app bundles in dir. A missing folder is not
// an error: the tab just has no Edge apps.
func ScanEdgeApps(dir string) []App {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []App{}
	}

	apps := []App{}
	for _, entry := range entries {
		path, ok := isBundle(dir, entry)
		if !ok {
			continue
		}

		// A web app whose plist cannot be read is still an app the user
		// installed: it keeps its folder name and opens by path.
		app, _ := InspectBundle(path)
		app.Source = SourceEdge

		if app.Description == "" {
			app.Description = "Edge"
		}

		apps = append(apps, app)
	}

	sortByName(apps)
	assignIDs(apps, PrefixEdge)

	return apps
}

// sortByName orders apps the way a list should read, ignoring case.
func sortByName(apps []App) {
	sort.SliceStable(apps, func(left, right int) bool {
		return strings.ToLower(apps[left].Name) < strings.ToLower(apps[right].Name)
	})
}
