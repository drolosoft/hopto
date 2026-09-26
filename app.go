package main

import (
	"context"
	"errors"
	"log"
	"os/exec"
	"sync"

	"github.com/drolosoft/hopto/internal/usage"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// The two tabs of the page. Each global shortcut opens the launcher on one
// of them; the page reports back which one the user is looking at.
const (
	tabApps  = "apps"
	tabLinks = "links"
)

// App exposes the catalogs to the page and owns the show/hide state of the
// launcher window.
type App struct {
	ctx      context.Context
	repoRoot string
	usage    *usage.Store

	mu      sync.Mutex
	visible bool
	tab     string
}

// NewApp creates the launcher; the repository root is detected once so the
// catalog can point at sibling builds while developing, and the usage file
// is loaded once. A broken usage file is logged and the launcher goes on
// read only, with empty counts, rather than refusing to start.
func NewApp() *App {
	store, err := usage.Open(usage.DefaultPath())
	if err != nil {
		log.Printf("usage: %v (favourites and counts are read only until the file is fixed)", err)
	}

	return &App{repoRoot: repoRootFromExecutable(), usage: store, tab: tabApps}
}

// startup stores the context and registers the global shortcuts. The window
// starts hidden; the shortcuts are the only way in, like Cmd+Tab.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	becomeAccessory()
	registerToggleHotkeys(a.toggle, mustHotkey(defaultAppsHotkey), mustHotkey(defaultLinksHotkey))
}

// mustHotkey parses a built-in spec; a typo there is a programming error,
// so it panics rather than starting without a shortcut.
func mustHotkey(spec string) Hotkey {
	hotkey, err := ParseHotkey(spec)
	if err != nil {
		panic(err)
	}

	return hotkey
}

// Apps returns the catalog with the resolved paths, so the page can grey out
// what is not built or installed.
func (a *App) Apps() []Entry {
	return resolve(a.repoRoot)
}

// AppCategories returns the filter chips of the apps tab.
func (a *App) AppCategories() []Category {
	return appCategories
}

// Links returns "mis links" with their hosts filled in.
func (a *App) Links() []Link {
	return linkCatalog.Links
}

// LinkCategories returns the filter chips of the links tab.
func (a *App) LinkCategories() []Category {
	return linkCatalog.Categories
}

// Usage returns the opening counts, last openings and favourites, so the
// page can sort by use and recency and mark the stars.
func (a *App) Usage() usage.Usage {
	return a.usage.Snapshot()
}

// ToggleFavorite marks or unmarks an item ("links:mdn", "apps:cronometro")
// and returns whether it is a favourite now.
func (a *App) ToggleFavorite(key string) (bool, error) {
	on, err := a.usage.ToggleFavorite(key)
	if err != nil {
		log.Printf("favorite %s: %v", key, err)
	}

	return on, err
}

// Toggle shows the launcher on the apps tab, or hides it when that tab is
// already showing. It is bound so the page can also call it.
func (a *App) Toggle() {
	a.toggle(tabApps)
}

// ShowLinks is Toggle for the links tab.
func (a *App) ShowLinks() {
	a.toggle(tabLinks)
}

// toggle is what both shortcuts end up in. Pressing the shortcut of the tab
// that is already on screen hides the launcher; the other one switches tab
// without hiding, so the two shortcuts also work as a way to jump between
// apps and links.
func (a *App) toggle(tab string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	log.Printf("toggle %s: visible=%v tab=%s", tab, a.visible, a.tab)

	if a.visible && a.tab == tab {
		runtime.WindowHide(a.ctx)
		a.visible = false
		return
	}

	if !a.visible {
		centerOnActiveScreen()
		runtime.WindowShow(a.ctx)
		activateApp()
		a.visible = true
	}

	a.tab = tab

	// The page resets its selection, search and filter on every appearance.
	runtime.EventsEmit(a.ctx, "shown", tab)
}

// TabChanged is called by the page when the user switches tab by hand, so
// the shortcuts know which tab is on screen.
func (a *App) TabChanged(tab string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.tab = tab
}

// Hide is called from the page on Escape or when the window loses focus.
func (a *App) Hide() {
	a.mu.Lock()
	defer a.mu.Unlock()

	runtime.WindowHide(a.ctx)
	a.visible = false
}

// Launch opens the app with the given id through `open`, which handles
// bundles the way a double click in the Finder does, then hides the launcher.
func (a *App) Launch(id string) error {
	for _, app := range resolve(a.repoRoot) {
		if app.ID != id {
			continue
		}

		if app.Path == "" {
			return errors.New(app.Name + " no está compilada ni instalada")
		}

		if err := exec.Command("open", app.Path).Run(); err != nil {
			return err
		}

		log.Printf("launched %s: %s", app.ID, app.Path)
		a.recordOpen(tabApps + ":" + app.ID)
		a.Hide()
		return nil
	}

	return errors.New("app desconocida: " + id)
}

// OpenLink opens the link with the given id in the default browser, through
// `open` again, then hides the launcher. Only catalog ids are accepted: the
// page never passes a URL, so nothing outside links.json can be opened.
func (a *App) OpenLink(id string) error {
	link, ok := findLink(id)
	if !ok {
		return errors.New("link desconocido: " + id)
	}

	if err := exec.Command("open", link.URL).Run(); err != nil {
		log.Printf("open %s: %v", link.URL, err)
		return err
	}

	log.Printf("opened link %s: %s", link.ID, link.URL)
	a.recordOpen(tabLinks + ":" + link.ID)
	a.Hide()
	return nil
}

// recordOpen counts an opening; a failure to save is logged, never shown,
// because the app or link did open.
func (a *App) recordOpen(key string) {
	if err := a.usage.RecordOpen(key); err != nil {
		log.Printf("usage: %v", err)
	}
}

// Debug writes a line from the page into the launcher log, so keyboard and
// focus problems in the WKWebView can be traced without an inspector.
func (a *App) Debug(message string) {
	log.Printf("page: %s", message)
}
