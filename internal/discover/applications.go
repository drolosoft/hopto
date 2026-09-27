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

// ScanApplications lists every .app one level under each root, plus the
// Utilities folder and any *.localized folder one level down. The same
// bundle reached from two roots, or through a symbolic link, is listed
// once, from the first root.
func ScanApplications(roots []string) []App {
	apps := []App{}
	seenPath := map[string]bool{}
	seenBundle := map[string]bool{}

	for _, root := range roots {
		for _, folder := range foldersToScan(root) {
			entries, err := os.ReadDir(folder)
			if err != nil {
				continue
			}

			for _, entry := range entries {
				path, ok := isBundle(folder, entry)
				if !ok || seenPath[path] {
					continue
				}

				seenPath[path] = true

				// A broken plist keeps the app in the list with its
				// folder name; the error is not worth a log line per
				// appearance.
				app, _ := InspectBundle(path)
				app.Source = SourceApplications

				if app.BundleID != "" {
					if seenBundle[app.BundleID] {
						continue
					}

					seenBundle[app.BundleID] = true
				}

				apps = append(apps, app)
			}
		}
	}

	sortByName(apps)
	assignIDs(apps, PrefixApplications)

	return apps
}

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
