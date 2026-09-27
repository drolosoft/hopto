package main

import (
	"context"
	"errors"
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
	// Center places the hidden window on a screen: mode is one of the
	// screen settings, display the CGDirectDisplayID for "last".
	Center(mode string, display uint32)
	Activate()
	Emit(name string, data any)
	SetClipboard(text string) error
	SetAlwaysOnTop(on bool)
	PickFile(directory string) (string, error)
	Quit()
}

// wailsWindow is the real window. Center and Activate go through the cgo
// helpers of hotkey_darwin.go, in the order the spec fixes.
type wailsWindow struct {
	ctx context.Context
}

func (w *wailsWindow) Show()     { runtime.WindowShow(w.ctx) }
func (w *wailsWindow) Hide()     { runtime.WindowHide(w.ctx) }
func (w *wailsWindow) Activate() { activateApp() }

func (w *wailsWindow) Emit(name string, data any) {
	runtime.EventsEmit(w.ctx, name, data)
}

// Center goes through the cgo helper of hotkey_darwin.go, which also
// sets the overlay behaviour of the window.
func (w *wailsWindow) Center(mode string, display uint32) {
	centerWindow(mode, display)
}

func (w *wailsWindow) SetClipboard(text string) error {
	return runtime.ClipboardSetText(w.ctx, text)
}

// SetAlwaysOnTop moves the window between the floating level and the
// normal one; the open panel needs the normal level to sit on top.
func (w *wailsWindow) SetAlwaysOnTop(on bool) {
	runtime.WindowSetAlwaysOnTop(w.ctx, on)
}

// PickFile runs the open panel as a sheet on this window, limited to
// .app bundles, and blocks until the user picks one ("" on cancel). It
// must be called from a goroutine, never from the main thread: Wails
// waits on a channel the sheet's completion handler fills.
func (w *wailsWindow) PickFile(directory string) (string, error) {
	return runtime.OpenFileDialog(w.ctx, runtime.OpenDialogOptions{
		DefaultDirectory: directory,
		Filters: []runtime.FileFilter{
			{DisplayName: "Applications", Pattern: "*.app"},
		},
	})
}

// Quit ends the app; the only way out besides pkill, from the menu.
func (w *wailsWindow) Quit() { runtime.Quit(w.ctx) }

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

	// menu is the menu bar item and login the LaunchAgent behind its
	// "Open at login" entry; the tests put fakes in both.
	menu  menuBar
	login loginAgent

	// background counts the icon fetches in flight, so the tests (and a
	// future clean shutdown) can wait for them.
	background sync.WaitGroup

	// offline skips the background icon fetches; the tests set it so a
	// unit test never reaches the network by accident.
	offline bool

	// display is the screen the panel was last shown on (a
	// CGDirectDisplayID, 0 for none yet), kept in window.json and
	// guarded by mu. The two functions read the real screens; the tests
	// replace them, since the real ones wait on the main thread.
	display          uint32
	windowDisplay    func() uint32
	attachedDisplays func() []uint32

	mu      sync.Mutex
	visible bool
	tab     string
	// dialogOpen is true while PickApp waits for the open panel, a sheet
	// on this window: hiding the window then would strand the sheet, and
	// Wails' dialog call with it, so toggle and Hide leave it alone.
	dialogOpen bool
	discovered map[string]discover.App
	fetched    map[string]bool

	// fetchGeneration counts the fetches started for an id, so a goroutine
	// started by an edit that is itself later overtaken (a second edit, a
	// delete) can tell its answer is stale and drop it instead of racing
	// the newer one to disk.
	fetchGeneration map[string]int

	// firstRun is true when this run wrote library.toml from the seed;
	// welcomeDismissed (under mu) hides the welcome for the rest of the
	// run; symbolicHotkeys is macOS's own shortcut table, read for the
	// Finder's ⌘⌥Space.
	firstRun         bool
	welcomeDismissed bool
	symbolicHotkeys  string
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
		window:   win,
		open:     open,
		home:     home,
		dataDir:  dataDir,
		language: language,
		scanner:  &discover.Scanner{},
		appRoots: []string{
			"/Applications", filepath.Join(home, "Applications"),
		},
		edgeDir: discover.EdgeAppsDir(home),
		login: loginAgent{
			path:       launchAgentPath(home),
			executable: os.Executable,
		},
		symbolicHotkeys: filepath.Join(home, symbolicHotkeysFile),
		tab:             tabApps,
		discovered:      map[string]discover.App{},
		fetched:         map[string]bool{},
		fetchGeneration: map[string]int{},
	}

	// Where the panel was last shown, and how to read the real screens.
	app.display = readWindowState(filepath.Join(dataDir, windowFile))
	app.windowDisplay = currentDisplay
	app.attachedDisplays = activeDisplays

	usageStore, err := usage.Open(filepath.Join(dataDir, usageFile))
	if err != nil {
		log.Printf(
			"usage: %v (favourites and counts are read only until the file is fixed)",
			err,
		)
	}
	app.usage = usageStore

	// No library file yet means hopto never ran for this user; Open is
	// about to write the seed, so the question is asked first.
	libraryPath := filepath.Join(dataDir, libraryFile)
	_, statErr := os.Stat(libraryPath)
	app.firstRun = errors.Is(statErr, os.ErrNotExist)

	store, err := library.Open(libraryPath, seed.For(language), home)
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
// the only way in, like Cmd+Tab. It also puts the item in the menu bar and
// answers a launch of the running app by showing the panel.
func (a *App) startup(ctx context.Context) {
	if a.window == nil {
		a.window = &wailsWindow{ctx: ctx}
	}

	becomeAccessory()

	apps, links := hotkeysFromSettings(a.library.Snapshot().Settings)
	registerToggleHotkeys(a.toggle, apps, links)

	// The menu bar item is the way to quit and to reach the help or the
	// file without the shortcuts.
	if a.menu == nil {
		a.menu = statusBar{}
	}

	a.installMenu()

	// A launch of the running app (Alfred, `open -a`, the Finder) shows
	// the panel like the shortcut does.
	handleReopen(a.showFromOutside)
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

	if a.dialogOpen {
		log.Printf("toggle ignored: a dialog is open")
		return
	}

	if a.visible && a.tab == tab {
		a.rememberDisplay()
		a.window.Hide()
		a.visible = false
		return
	}

	if !a.visible {
		a.placeWindow()
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
// and by every successful opening. While a dialog is open it does
// nothing: the page's blur fires as the sheet takes the focus. Before
// hiding it notes the display the window is on, for the next show.
func (a *App) Hide() {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.dialogOpen {
		return
	}

	if a.visible {
		a.rememberDisplay()
	}

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
