package discover

import (
	"os"

	"github.com/drolosoft/hopto/internal/library"
)

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
	assignIDs(apps, library.PrefixApplications)

	return apps
}
