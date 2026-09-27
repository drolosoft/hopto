package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/drolosoft/hopto/internal/discover"
	"github.com/drolosoft/hopto/internal/icons"
	"github.com/drolosoft/hopto/internal/library"
	"github.com/drolosoft/hopto/internal/usage"
)

// Kinds and sources of an item as the page sees them.
const (
	kindApp       = "app"
	kindLink      = "link"
	sourceLibrary = "library"
)

// hiddenChip is the virtual chip of the apps hidden with ⌘⌫; library
// reserves the id, so no category of the user can take it.
const hiddenChip = "hidden"

// ItemView is one row or card of the page: a link, a hand-added app or a
// discovered one, flattened so the page never branches on where it came
// from. Every slice is non-nil. SearchOnly marks an app of
// /System/Applications: the page lists it only while typing.
type ItemView struct {
	ID          string   `json:"id"`
	Key         string   `json:"key"`
	Kind        string   `json:"kind"`
	Source      string   `json:"source"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	URL         string   `json:"url"`
	Host        string   `json:"host"`
	Path        string   `json:"path"`
	BundleID    string   `json:"bundleId"`
	Category    string   `json:"category"`
	Keywords    []string `json:"keywords"`
	IconURL     string   `json:"iconUrl"`
	Hidden      bool     `json:"hidden"`
	Missing     bool     `json:"missing"`
	SearchOnly  bool     `json:"searchOnly"`
}

// CategoryView is one chip. A virtual one groups discovered apps and has
// no name: the page translates its id.
type CategoryView struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Tab     string `json:"tab"`
	Virtual bool   `json:"virtual"`
}

// Ref points the page at an existing item ("you already have this one").
type Ref struct {
	Tab  string `json:"tab"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

// SettingsView is the library's settings with the language resolved.
type SettingsView struct {
	Language              string   `json:"language"`
	HotkeyApps            string   `json:"hotkeyApps"`
	HotkeyLinks           string   `json:"hotkeyLinks"`
	SecondaryBrowser      string   `json:"secondaryBrowser"`
	ScanApplications      bool     `json:"scanApplications"`
	DiscoverEdgeApps      bool     `json:"discoverEdgeApps"`
	IconServices          []string `json:"iconServices"`
	AllowPrivateIconHosts bool     `json:"allowPrivateIconHosts"`
}

// StatusView tells the page whether the library file is usable.
type StatusView struct {
	Path     string `json:"path"`
	Error    string `json:"error"`
	Line     int    `json:"line"`
	ReadOnly bool   `json:"readOnly"`
}

// Items returns the apps or the links, after re-reading the library if it
// changed on disk. Called on every appearance of the panel.
func (a *App) Items(tab string) []ItemView {
	a.reload()
	lib := a.library.Snapshot()

	if tab == tabLinks {
		return a.linkViews(lib)
	}

	return a.appViews(lib)
}

// Categories returns the chips of a tab: the library's, plus the virtual
// ones for the discovered apps that are actually present, and the hidden
// apps' chip when there is any.
func (a *App) Categories(tab string) []CategoryView {
	a.reload()
	lib := a.library.Snapshot()

	views := []CategoryView{}
	for _, category := range lib.Categories {
		if category.Tab == tab {
			views = append(
				views, CategoryView{ID: category.ID, Name: category.Name, Tab: tab},
			)
		}
	}

	if tab != tabApps {
		return views
	}

	hidden := map[string]bool{}
	for _, entry := range lib.Hidden {
		hidden[entry.ID] = true
	}

	present := map[string]bool{}
	anyHidden := false
	for _, app := range a.discover(lib.Settings) {
		// A search-only app is never on the grid, so it cannot be the
		// reason its chip exists.
		if !a.searchOnly(app.Path) {
			present[app.Source] = true
		}

		anyHidden = anyHidden || hidden[app.ID]
	}

	sources := []string{discover.SourceApplications, discover.SourceEdge}
	for _, source := range sources {
		if present[source] {
			views = append(views, CategoryView{ID: source, Tab: tab, Virtual: true})
		}
	}

	// The hidden apps get a chip only while there is one on disk: an
	// empty chip would be a way to nothing. It is the only place they
	// show, and where ⌘⌫ brings one back.
	if anyHidden {
		views = append(views, CategoryView{ID: hiddenChip, Tab: tab, Virtual: true})
	}

	return views
}

// Usage returns the opening counts, last openings and favourites.
func (a *App) Usage() usage.Usage {
	return a.usage.Snapshot()
}

// Settings returns the settings with "auto" resolved to a real language.
func (a *App) Settings() SettingsView {
	a.reload()
	settings := a.library.Snapshot().Settings

	return SettingsView{
		Language:              languageFor(settings.Language, a.language),
		HotkeyApps:            settings.HotkeyApps,
		HotkeyLinks:           settings.HotkeyLinks,
		SecondaryBrowser:      settings.SecondaryBrowser,
		ScanApplications:      settings.ScanApplications,
		DiscoverEdgeApps:      settings.DiscoverEdgeApps,
		IconServices:          append([]string{}, settings.IconServices...),
		AllowPrivateIconHosts: settings.AllowPrivateIconHosts,
	}
}

// LibraryStatus reports where the file is and, when it cannot be used,
// why and on which line.
func (a *App) LibraryStatus() StatusView {
	status := a.library.Status()

	return StatusView{
		Path:     status.Path,
		Error:    status.Error,
		Line:     status.Line,
		ReadOnly: status.ReadOnly,
	}
}

// reload re-reads the library when it changed on disk; a broken edit is
// logged and keeps the last good version on screen.
func (a *App) reload() {
	if err := a.library.ReloadIfChanged(); err != nil {
		log.Printf("library: %v", err)
	}
}

// linkViews flattens the links and starts fetching the icons that are
// missing (once per id per run; see app_edit.go).
func (a *App) linkViews(lib library.Library) []ItemView {
	views := make([]ItemView, 0, len(lib.Links))

	for _, link := range lib.Links {
		host, _ := library.HostOf(link.URL)

		views = append(views, ItemView{
			ID:          link.ID,
			Key:         tabLinks + ":" + link.ID,
			Kind:        kindLink,
			Source:      sourceLibrary,
			Name:        link.Name,
			Description: link.Description,
			URL:         link.URL,
			Host:        host,
			Category:    link.Category,
			Keywords:    append([]string{}, link.Keywords...),
			IconURL:     icons.FileURL(a.iconsDir(), link.ID),
		})
	}

	a.fetchMissingIcons(lib.Links)

	return views
}

// appViews lists the hand-added apps first, then the discovered ones,
// flagging the hidden ones instead of dropping them so the page can offer
// to unhide.
func (a *App) appViews(lib library.Library) []ItemView {
	hidden := map[string]bool{}
	for _, entry := range lib.Hidden {
		hidden[entry.ID] = true
	}

	views := []ItemView{}

	for _, app := range lib.Apps {
		view := ItemView{
			ID:          app.ID,
			Key:         tabApps + ":" + app.ID,
			Kind:        kindApp,
			Source:      sourceLibrary,
			Name:        app.Name,
			Description: app.Description,
			Path:        app.Path,
			BundleID:    app.BundleID,
			Category:    app.Category,
			Keywords:    []string{},
			IconURL:     a.iconURL(app.ID),
		}

		if app.Path != "" {
			if _, err := os.Stat(app.Path); err != nil {
				view.Missing = true
			}
		}

		views = append(views, view)
	}

	for _, app := range a.discover(lib.Settings) {
		views = append(views, ItemView{
			ID:          app.ID,
			Key:         tabApps + ":" + app.ID,
			Kind:        kindApp,
			Source:      app.Source,
			Name:        app.Name,
			Description: app.Description,
			Host:        app.Host,
			URL:         app.URL,
			Path:        app.Path,
			BundleID:    app.BundleID,
			Category:    app.Source,
			Keywords:    []string{},
			IconURL:     a.iconURL(app.ID),
			Hidden:      hidden[app.ID],
			SearchOnly:  a.searchOnly(app.Path),
		})
	}

	return views
}

// discover runs the scans the settings allow and remembers the result, so
// Launch and the icon handler can find a discovered app by id.
func (a *App) discover(settings library.Settings) []discover.App {
	found := []discover.App{}

	if settings.DiscoverEdgeApps {
		found = append(found, discover.ScanEdgeApps(a.edgeDir)...)
	}

	if settings.ScanApplications {
		found = append(found, a.scanner.Applications(a.appRoots)...)
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	a.discovered = make(map[string]discover.App, len(found))
	for _, app := range found {
		a.discovered[app.ID] = app
	}

	return found
}

// iconURL is the page URL of an item's icon: the user's own file when
// there is one, else the bundle's .icns through the handler, else "".
func (a *App) iconURL(id string) string {
	if url := icons.FileURL(a.iconsDir(), id); url != "" {
		return url
	}

	icnsPath, ok := a.iconSource(id)
	if !ok {
		return ""
	}

	info, err := os.Stat(icnsPath)
	if err != nil {
		return ""
	}

	// Nanoseconds, not seconds: see the comment on icons.FileURL.
	return icons.StampURL(id, info.ModTime().UnixNano())
}

// iconSource is the handler's resolver: the .icns behind an app id, from
// the last discovery or from a hand-added app's bundle.
func (a *App) iconSource(id string) (string, bool) {
	a.mu.Lock()
	found, ok := a.discovered[id]
	a.mu.Unlock()

	if ok {
		return found.IconPath, found.IconPath != ""
	}

	for _, app := range a.library.Snapshot().Apps {
		if app.ID != id || app.Path == "" {
			continue
		}

		bundle, _ := discover.InspectBundle(app.Path)

		return bundle.IconPath, bundle.IconPath != ""
	}

	return "", false
}

// searchOnly reports whether a discovered app lives under the system's
// own apps folder.
func (a *App) searchOnly(path string) bool {
	return a.systemApps != "" &&
		strings.HasPrefix(path, a.systemApps+string(filepath.Separator))
}
