package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/drolosoft/hopto/internal/discover"
	"github.com/drolosoft/hopto/internal/icons"
	"github.com/drolosoft/hopto/internal/library"
	"github.com/drolosoft/hopto/internal/usage"
	"github.com/drolosoft/hopto/seed"
)

// The two tabs of the page. Each global shortcut opens the launcher on one
// of them; the page reports back which one the user is looking at.
const (
	tabApps  = library.TabApps
	tabLinks = library.TabLinks
)

// maxDebugRunes caps a line the page sends to the log; the page is ours,
// but a runaway loop there must not fill the disk.
const maxDebugRunes = 500

// window is what App needs from the Wails window, behind an interface so
// the tests can run toggle, Launch and OpenLink without a display.
type window interface {
	Show()
	Hide()
	Center()
	Activate()
	Emit(name string, data any)
	SetClipboard(text string) error
}

// wailsWindow is the real window. Center and Activate go through the cgo
// helpers of hotkey_darwin.go, in the order the spec fixes.
type wailsWindow struct {
	ctx context.Context
}

func (w *wailsWindow) Show()     { runtime.WindowShow(w.ctx) }
func (w *wailsWindow) Hide()     { runtime.WindowHide(w.ctx) }
func (w *wailsWindow) Center()   { centerOnActiveScreen() }
func (w *wailsWindow) Activate() { activateApp() }

func (w *wailsWindow) Emit(name string, data any) {
	runtime.EventsEmit(w.ctx, name, data)
}

func (w *wailsWindow) SetClipboard(text string) error {
	return runtime.ClipboardSetText(w.ctx, text)
}

// runOpen is /usr/bin/open, the only way hopto starts anything: it handles
// bundles and URLs the way a double click in the Finder does.
func runOpen(args ...string) error {
	return exec.Command("/usr/bin/open", args...).Run()
}

// App is what the page talks to. It owns the show/hide state of the window
// and translates between the page and the packages under internal/.
type App struct {
	window   window
	open     func(args ...string) error
	home     string
	dataDir  string
	language string

	library     *library.Store
	usage       *usage.Store
	scanner     *discover.Scanner
	iconHandler *icons.Handler

	// Where discovery looks; the tests point them inside a temp home.
	appRoots []string
	edgeDir  string

	// background counts the icon fetches in flight, so the tests (and a
	// future clean shutdown) can wait for them.
	background sync.WaitGroup

	// offline skips the background icon fetches; the tests set it so a
	// unit test never reaches the network by accident.
	offline bool

	mu         sync.Mutex
	visible    bool
	tab        string
	discovered map[string]discover.App
	fetched    map[string]bool
}

// NewApp builds the launcher for the real Mac: the user's home, the system
// language and the real window and `open`.
func NewApp() *App {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("home folder: %v", err)
	}

	return newApp(home, dataDir(home), nil, runOpen, systemLanguage())
}

// newApp wires the stores and the icon handler. A broken usage or library
// file is logged and the launcher goes on read only rather than refusing
// to start; the window may be nil until startup hands over the context.
func newApp(
	home, dataDir string, win window, open func(args ...string) error,
	language string,
) *App {
	app := &App{
		window:     win,
		open:       open,
		home:       home,
		dataDir:    dataDir,
		language:   language,
		scanner:    &discover.Scanner{},
		appRoots:   []string{"/Applications", filepath.Join(home, "Applications")},
		edgeDir:    discover.EdgeAppsDir(home),
		tab:        tabApps,
		discovered: map[string]discover.App{},
		fetched:    map[string]bool{},
	}

	usageStore, err := usage.Open(filepath.Join(dataDir, usageFile))
	if err != nil {
		log.Printf(
			"usage: %v (favourites and counts are read only until the file is fixed)",
			err,
		)
	}
	app.usage = usageStore

	store, err := library.Open(
		filepath.Join(dataDir, libraryFile), seed.For(language), home,
	)
	if err != nil {
		log.Printf(
			"library: %v (the panel shows the last good version, edits are refused)",
			err,
		)
	}
	app.library = store

	handler, err := icons.NewHandler(app.iconsDir(), app.iconSource)
	if err != nil {
		log.Printf("icons: %v (icons will not be served)", err)
	}
	app.iconHandler = handler

	return app
}

// startup stores the context, leaves the Dock and registers the global
// shortcuts from the library. The window starts hidden; the shortcuts are
// the only way in, like Cmd+Tab.
func (a *App) startup(ctx context.Context) {
	if a.window == nil {
		a.window = &wailsWindow{ctx: ctx}
	}

	becomeAccessory()

	apps, links := hotkeysFromSettings(a.library.Snapshot().Settings)
	registerToggleHotkeys(a.toggle, apps, links)
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
		a.window.Hide()
		a.visible = false
		return
	}

	if !a.visible {
		a.window.Center()
		a.window.Show()
		a.window.Activate()
		a.visible = true
	}

	a.tab = tab

	// The page resets its selection, search and filter on every appearance.
	a.window.Emit("shown", tab)
}

// TabChanged is called by the page when the user switches tab by hand, so
// the shortcuts know which tab is on screen.
func (a *App) TabChanged(tab string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if tab == tabLinks {
		a.tab = tabLinks
	} else {
		a.tab = tabApps
	}
}

// Hide is called from the page on Escape or when the window loses focus,
// and by every successful opening.
func (a *App) Hide() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.window.Hide()
	a.visible = false
}

// emit sends an event to the page when there is a window to send it to
// (the background fetches may finish before startup, or in a test): a
// finished icon download tells the page through this, in app_edit.go.
func (a *App) emit(name string, data any) {
	if a.window != nil {
		a.window.Emit(name, data)
	}
}

// Debug writes a line from the page into the launcher log, so keyboard and
// focus problems in the WKWebView can be traced without an inspector.
func (a *App) Debug(message string) {
	log.Printf("page: %s", debugLine(message))
}

// debugLine strips control characters and cuts the line to maxDebugRunes.
func debugLine(message string) string {
	clean := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}

		return r
	}, message)

	runes := []rune(clean)
	if len(runes) > maxDebugRunes {
		runes = runes[:maxDebugRunes]
	}

	return string(runes)
}

// assets is the fallback handler of the Wails asset server: the icons, or
// nothing when the icons folder could not be opened.
func (a *App) assets() http.Handler {
	if a.iconHandler == nil {
		return http.NotFoundHandler()
	}

	return a.iconHandler
}
