//go:build !windows

package discover

import (
	"os"
	"path/filepath"
	"strings"
)

// utilitiesFolder is the one subfolder of /Applications macOS itself fills.
const utilitiesFolder = "Utilities"

// pwaFolderSuffix marks the folders browsers keep their web apps in ("Edge
// Apps.localized", "Chrome Apps.localized"); the Edge scan owns those.
const pwaFolderSuffix = " Apps.localized"

// foldersToScan is the root itself and its known subfolders.
func foldersToScan(root string) []string {
	folders := []string{root, filepath.Join(root, utilitiesFolder)}

	entries, err := os.ReadDir(root)
	if err != nil {
		return folders
	}

	for _, entry := range entries {
		name := entry.Name()
		isLocalized := entry.IsDir() &&
			strings.HasSuffix(name, ".localized") &&
			!strings.HasSuffix(name, pwaFolderSuffix)

		if isLocalized {
			folders = append(folders, filepath.Join(root, name))
		}
	}

	return folders
}

// scanRoots is what the Scanner walks on this platform.
func scanRoots(roots []string) []App {
	return ScanApplications(roots)
}

// ScanEdgeApps lists the web app bundles in dir (scanEdgeBundles has the
// body).
func ScanEdgeApps(dir string) []App {
	return scanEdgeBundles(dir)
}

// scanEdgeBundles lists the web app bundles in dir. A missing folder is
// not an error: the tab just has no Edge apps.
func scanEdgeBundles(dir string) []App {
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
