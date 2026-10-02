// Package library is the user's catalog of hopto: the categories, the links,
// the apps added by hand, the apps hidden from the scan and the settings,
// all in one TOML file the app writes and the user may edit. The rules of
// the file live in validate.go; reading and writing it, in store.go.
package library

// CurrentVersion is the schema of library.toml this build writes. A file
// with a higher number comes from a newer hopto and is never rewritten.
const CurrentVersion = 1

// The two tabs; a category belongs to one of them.
const (
	TabApps  = "apps"
	TabLinks = "links"
)

// Library is the whole file. Field order is the order the encoder writes.
type Library struct {
	Version    int        `toml:"version"`
	Settings   Settings   `toml:"settings"`
	Categories []Category `toml:"categories,omitempty"`
	Links      []Link     `toml:"links,omitempty"`
	Apps       []AppEntry `toml:"apps,omitempty"`
	Hidden     []Hidden   `toml:"hidden,omitempty"`
}

// Settings are the few knobs of hopto. Language "auto" follows the system;
// the hotkeys are specs like "cmd+shift+space" parsed by the app; screen
// is where the panel appears ("last": the display it was last shown on,
// else the main one; "mouse": the one under the pointer; "main"); the
// icon services are third parties asked for a favicon, in order, and are
// opt-in because they receive the host of every link; the secondary
// browser is the bundle id ⌘↩ opens links with; and
// allow_private_icon_hosts lets the icon fetch reach loopback and private
// addresses.
type Settings struct {
	Language              string   `toml:"language"`
	HotkeyApps            string   `toml:"hotkey_apps"`
	HotkeyLinks           string   `toml:"hotkey_links"`
	Screen                string   `toml:"screen"`
	ScanApplications      bool     `toml:"scan_applications"`
	DiscoverEdgeApps      bool     `toml:"discover_edge_apps"`
	IconServices          []string `toml:"icon_services"`
	SecondaryBrowser      string   `toml:"secondary_browser,omitempty"`
	AllowPrivateIconHosts bool     `toml:"allow_private_icon_hosts"`
}

// Category is one filter chip. The array order in the file is the order of
// the chips on screen.
type Category struct {
	ID   string `toml:"id"`
	Name string `toml:"name"`
	Tab  string `toml:"tab"`
}

// Link is one entry of the links tab. Keywords join the search text; Icon
// is a hint of where to fetch the icon from ("sh:<name>" for a selfh.st
// icon, or an https URL), the PNG itself lives in icons/<id>.png.
type Link struct {
	ID          string   `toml:"id"`
	Name        string   `toml:"name"`
	Description string   `toml:"description,omitempty"`
	URL         string   `toml:"url"`
	Category    string   `toml:"category"`
	Keywords    []string `toml:"keywords,omitempty"`
	Icon        string   `toml:"icon,omitempty"`
}

// AppEntry is an app added by hand (the scanned ones are not stored). It is
// opened by bundle id when there is one, which survives moving the .app,
// and by path otherwise. Not called App: that name is the Wails struct.
type AppEntry struct {
	ID          string `toml:"id"`
	Name        string `toml:"name"`
	Description string `toml:"description,omitempty"`
	BundleID    string `toml:"bundle_id,omitempty"`
	Path        string `toml:"path,omitempty"`
	Category    string `toml:"category"`
}

// Hidden names a discovered app (an "app-" or "edge-" id) the user does
// not want in the list.
type Hidden struct {
	ID string `toml:"id"`
}

// Default is an empty library with the settings hopto starts with.
func Default() Library {
	return Library{
		Version: CurrentVersion,
		Settings: Settings{
			Language:         "auto",
			HotkeyApps:       rules.defaultAppsHotkey,
			HotkeyLinks:      rules.defaultLinksHotkey,
			Screen:           "last",
			ScanApplications: true,
			DiscoverEdgeApps: true,
			IconServices:     []string{"site"},
		},
		Categories: []Category{},
		Links:      []Link{},
		Apps:       []AppEntry{},
		Hidden:     []Hidden{},
	}
}
