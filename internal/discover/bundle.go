// Package discover lists the apps that are on the Mac without the user
// adding them: the Edge web apps under ~/Applications, and every .app in
// /Applications and ~/Applications. It reads each bundle's Info.plist for
// the bundle id, the icon file and, for a web app, its URL.
package discover

import (
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"

	"github.com/drolosoft/hopto/internal/library"
)

// Where a discovered app came from; the page shows it as a chip.
const (
	SourceApplications = "applications"
	SourceEdge         = "edge"
)

// The id prefixes reserved for discovered apps (library.CheckID refuses
// them for user entries).
const (
	PrefixApplications = "app-"
	PrefixEdge         = "edge-"
)

// App is one discovered application. IconPath is the .icns inside the
// bundle, "" when the icon lives in an asset catalog hopto cannot read.
type App struct {
	ID          string
	Name        string
	Description string
	Path        string
	BundleID    string
	IconPath    string
	Host        string

	// URL is the start page of an Edge web app, "" for any other app: the
	// page pairs a web app with a link only when both open this page.
	URL string

	Source string
}

// bundleInfo is the part of Info.plist hopto reads. The Cr* keys are what
// Edge (and Chrome) write into a web app's bundle.
type bundleInfo struct {
	Identifier string `plist:"CFBundleIdentifier"`
	IconFile   string `plist:"CFBundleIconFile"`
	EdgeName   string `plist:"CrAppModeShortcutName"`
	EdgeURL    string `plist:"CrAppModeShortcutURL"`
}

// InspectBundle reads a .app folder. The name is the folder name, which is
// what the Finder shows; the plist adds the bundle id, the icon and, for a
// web app, the URL. The returned App is usable even when the error is not
// nil: a bundle with a broken plist still has a name and a path.
func InspectBundle(path string) (App, error) {
	app := App{
		Path:   path,
		Name:   strings.TrimSuffix(filepath.Base(path), ".app"),
		Source: SourceApplications,
	}

	infoPath := filepath.Join(path, "Contents", "Info.plist")
	data, err := os.ReadFile(infoPath)
	if err != nil {
		return app, err
	}

	var info bundleInfo
	if _, err := plist.Unmarshal(data, &info); err != nil {
		return app, err
	}

	app.BundleID = info.Identifier
	app.IconPath = iconPath(path, info.IconFile)

	if info.EdgeURL != "" {
		app.Source = SourceEdge

		// The host is enough as description: a full URL overflows the
		// card.
		if host, err := library.HostOf(info.EdgeURL); err == nil {
			app.Host = host
			app.Description = host
			app.URL = info.EdgeURL
		}
	}

	return app, nil
}

// iconPath resolves CFBundleIconFile, which may or may not carry the
// .icns extension, to an existing file under Resources.
func iconPath(bundle, iconFile string) string {
	if iconFile == "" {
		return ""
	}

	if !strings.HasSuffix(iconFile, ".icns") {
		iconFile += ".icns"
	}

	candidate := filepath.Join(bundle, "Contents", "Resources", iconFile)
	if info, err := os.Stat(candidate); err != nil || !info.Mode().IsRegular() {
		return ""
	}

	return candidate
}

// isBundle reports whether a directory entry is a .app folder, following a
// symbolic link to one so the same app reached two ways is seen once. An
// ordinary folder keeps the path it was found at instead of its resolved
// form: macOS itself keeps /var as a symlink to /private/var, so resolving
// every entry would silently rewrite a path under a temporary directory
// (or under /var/folders in general) into its /private form.
func isBundle(dir string, entry os.DirEntry) (string, bool) {
	if !strings.HasSuffix(entry.Name(), ".app") {
		return "", false
	}

	path := filepath.Join(dir, entry.Name())

	if entry.Type()&os.ModeSymlink == 0 {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return "", false
		}

		return path, true
	}

	real, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", false
	}

	info, err := os.Stat(real)
	if err != nil || !info.IsDir() {
		return "", false
	}

	return real, true
}

// assignIDs gives every app an id from its name with the given prefix,
// unique within the list. Apps are visited in the order given, so callers
// sort first to keep ids stable between scans.
func assignIDs(apps []App, prefix string) {
	taken := map[string]bool{}

	for index := range apps {
		base := prefix + library.Slug(apps[index].Name)
		id := library.UniqueID(base, func(candidate string) bool {
			return taken[candidate]
		})

		taken[id] = true
		apps[index].ID = id
	}
}
