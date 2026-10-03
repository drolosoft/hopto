// Package app is the launcher behind the page: the App that Wails binds,
// with its views, editor, opening, menu, welcome and window placement.
// It talks to the operating system through internal/platform and to the
// shortcuts, the menu bar item and the tray through internal/native.
package app

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/drolosoft/hopto/internal/discover"
	"github.com/drolosoft/hopto/internal/icons"
	"github.com/drolosoft/hopto/internal/library"
	"github.com/drolosoft/hopto/internal/native"
	"github.com/drolosoft/hopto/internal/platform"
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
	// screen settings, display the native package's id of the screen
	// for "last".
	Center(mode string, display uint32)
	Activate()
	Emit(name string, data any)
	SetClipboard(text string) error
	SetAlwaysOnTop(on bool)
	PickFile(directory string) (string, error)
	Quit()
}

// wailsWindow is the real window. Center and Activate go through the
// native package, since Wails has neither; showLocked calls them in
// their order.
type wailsWindow struct {
	ctx context.Context
}

// Show makes the window visible; Center has already placed it.
func (w *wailsWindow) Show() { runtime.WindowShow(w.ctx) }

// Hide takes the window off screen without quitting.
func (w *wailsWindow) Hide() { runtime.WindowHide(w.ctx) }

// Activate gives hopto the keyboard, which an accessory app does not
// get on its own when its window shows.
func (w *wailsWindow) Activate() { native.Activate() }

// Emit sends an event to the page.
func (w *wailsWindow) Emit(name string, data any) {
	runtime.EventsEmit(w.ctx, name, data)
}

// Center goes through native.CenterWindow, which on macOS also sets the
// overlay behaviour of the window.
func (w *wailsWindow) Center(mode string, display uint32) {
	native.CenterWindow(mode, display)
}

// SetClipboard puts text on the system clipboard.
func (w *wailsWindow) SetClipboard(text string) error {
	return runtime.ClipboardSetText(w.ctx, text)
}

// SetAlwaysOnTop moves the window in and out of always on top; the file
// dialog needs it out to sit on top of the window.
func (w *wailsWindow) SetAlwaysOnTop(on bool) {
	runtime.WindowSetAlwaysOnTop(w.ctx, on)
}

// PickFile runs the system's file dialog on this window, filtered by
// platform.PickFilters, and blocks until the user picks a file ("" on
// cancel). It must be called from a goroutine, never from the main
// thread: on macOS the dialog is a sheet, and Wails waits on a channel
// its completion handler fills.
func (w *wailsWindow) PickFile(directory string) (string, error) {
	return runtime.OpenFileDialog(w.ctx, runtime.OpenDialogOptions{
		DefaultDirectory: directory,
		Filters:          wailsFilters(platform.PickFilters()),
	})
}

// wailsFilters turns the platform's file filters into Wails' type: the
// platform package must not import the Wails runtime.
func wailsFilters(filters []platform.FileFilter) []runtime.FileFilter {
	out := make([]runtime.FileFilter, 0, len(filters))

	for _, filter := range filters {
		out = append(out, runtime.FileFilter{
			DisplayName: filter.Name, Pattern: filter.Pattern,
		})
	}

	return out
}

// Quit ends the app, for the menu's Quit entry: closing the window only
// hides it.
func (w *wailsWindow) Quit() { runtime.Quit(w.ctx) }

// App is what the page talks to. It owns the show/hide state of the window
// and translates between the page and the packages under internal/.
//
// Every field down to mu is set before the App is shared (by newApp, or
// by startup for window and menu, before any shortcut can fire) and only
// read afterwards; the fields after mu change while the app runs, and mu
// guards them.
type App struct {
	// window is the Wails window, or a fake in the tests; startup sets
	// it when newApp was given none.
	window window

	// open runs `open`'s arguments on the system (platform.RunOpen);
	// the tests record them instead.
	open func(args ...string) error

	// home is the user's home folder, which library paths may start from.
	home string

	// dataDir holds library.toml, the usage and window files and icons/.
	dataDir string

	// language is the system's, used where the settings say "auto".
	language string

	// library is library.toml; the store takes its own lock.
	library *library.Store

	// usage holds the opening counts and favourites; the store takes its
	// own lock.
	usage *usage.Store

	// scanner lists the installed apps; it takes its own lock.
	scanner *discover.Scanner

	// iconHandler serves the icons to the webview, nil when the icons
	// folder could not be opened.
	iconHandler *icons.Handler

	// appRoots are the folders discovery scans; the tests point them, and
	// the two fields after it, inside a temporary home.
	appRoots []string

	// edgeDir is where Edge keeps its web apps.
	edgeDir string

	// systemApps is the last of the roots, the one whose apps are only
	// found by typing.
	systemApps string

	// menu is the menu bar item (the tray icon on Windows); the tests
	// put a fake in it.
	menu menuBar

	// login is the switch behind the menu's "Open at login" entry; the
	// tests put a fake in it.
	login platform.LoginAgent

	// background counts the icon fetches in flight, so the tests (and a
	// future clean shutdown) can wait for them; it is its own lock.
	background sync.WaitGroup

	// offline skips the background icon fetches; the tests set it so a
	// unit test never reaches the network by accident.
	offline bool

	// windowDisplay reads the screen the window is on now. The tests
	// replace it, since the real one waits on the main thread.
	windowDisplay func() uint32

	// attachedDisplays lists the screens attached now; replaced in the
	// tests for the same reason.
	attachedDisplays func() []uint32

	// firstRun is true when this run wrote library.toml from the seed.
	firstRun bool

	// symbolicHotkeysPath is the file of macOS's own shortcut table, read
	// for the Finder's ⌘⌥Space; never read on Windows, which has none.
	symbolicHotkeysPath string

	// mu guards every field below it.
	mu sync.Mutex

	// visible is whether the panel is on screen.
	visible bool

	// tab is the tab the page shows, apps or links.
	tab string

	// dialogOpen is true while PickApp waits for the file dialog on this
	// window: hiding the window then would strand the dialog, and Wails'
	// dialog call with it, so toggle and Hide leave it alone.
	dialogOpen bool

	// discovered is the last discovery by id, for Launch and the icon
	// handler.
	discovered map[string]discover.App

	// fetched marks the ids whose icon fetch has started in this run, so
	// a site is asked once per run.
	fetched map[string]bool

	// fetchGeneration counts the fetches started for an id, so a goroutine
	// started by an edit that is itself later overtaken (a second edit, a
	// delete) can tell its answer is stale and drop it instead of racing
	// the newer one to disk.
	fetchGeneration map[string]int

	// display is the screen the panel was last shown on (the native
	// package's id for it, 0 for none yet), kept in window.json.
	display uint32

	// welcomeDismissed hides the welcome for the rest of the run.
	welcomeDismissed bool
}

// Wiring is what main hands to Wails besides the App itself: the
// lifecycle hooks and the asset handler. They are not page API, and an
// exported method on App would be bound to the page, so they travel
// here instead.
type Wiring struct {
	// Startup is the OnStartup hook.
	Startup func(ctx context.Context)

	// SecondInstance answers a second launch of the single-instance app.
	SecondInstance func(data options.SecondInstanceData)

	// Assets serves the page's icons and files to the webview.
	Assets http.Handler
}

// New builds the launcher for the real desktop: the user's home, the system
// language and the real window and `open`, plus the wiring main needs
// for it. The asset handler is built here, once: newApp has already
// opened the icons, so it does not wait for startup.
func New() (*App, Wiring) {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Printf("home folder: %v", err)
	}

	launcher := newApp(
		home, platform.DataDir(home), nil, platform.RunOpen,
		platform.SystemLanguage(),
	)

	return launcher, Wiring{
		Startup:        launcher.startup,
		SecondInstance: launcher.secondInstance,
		Assets:         launcher.assets(),
	}
}

// newApp wires the stores and the icon handler. A broken usage or library
// file is logged and the launcher goes on read only rather than refusing
// to start; the window may be nil until startup hands over the context.
func newApp(
	home, dataDir string, win window, open func(args ...string) error,
	language string,
) *App {
	app := &App{
		window:              win,
		open:                open,
		home:                home,
		dataDir:             dataDir,
		language:            language,
		scanner:             &discover.Scanner{},
		appRoots:            platform.AppRoots(home),
		edgeDir:             platform.EdgeAppsDir(home),
		systemApps:          platform.SystemAppsRoot(),
		login:               platform.NewLoginAgent(home),
		symbolicHotkeysPath: filepath.Join(home, platform.SymbolicHotkeysFile),
		tab:                 tabApps,
		discovered:          map[string]discover.App{},
		fetched:             map[string]bool{},
		fetchGeneration:     map[string]int{},
	}

	// Where the panel was last shown, and how to read the real screens.
	app.display = readWindowState(filepath.Join(dataDir, platform.WindowFile))
	app.windowDisplay = native.CurrentDisplay
	app.attachedDisplays = native.ActiveDisplays

	usageStore, err := usage.Open(filepath.Join(dataDir, platform.UsageFile))
	if err != nil {
		log.Printf(
			"usage: %v (favourites and counts are read only until the file is fixed)",
			err,
		)
	}
	app.usage = usageStore

	// No library file yet means hopto never ran for this user; Open is
	// about to write the seed, so the question is asked first.
	libraryPath := filepath.Join(dataDir, platform.LibraryFile)
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

// startup stores the context, leaves the Dock on macOS and registers the
// global shortcuts from the library. The window starts hidden; the
// shortcuts are the main way in, like Cmd+Tab. It also puts the item in
// the menu bar (the notification area on Windows) and answers a launch of
// the running app by showing the panel.
func (a *App) startup(ctx context.Context) {
	if a.window == nil {
		a.window = &wailsWindow{ctx: ctx}
	}

	native.BecomeAccessory()

	// The native side reaches the App only through these, so they are
	// handed over before anything that can fire them is registered.
	native.Start(native.Hooks{
		Toggle:           a.toggle,
		Reopen:           a.showFromOutside,
		MenuPicked:       a.menuAction,
		HotkeyRegistered: recordHotkeyStatus,
	})

	apps, links := platform.HotkeysFromSettings(a.library.Snapshot().Settings)
	native.RegisterHotkeys(apps, links)

	// The menu bar item is the way to quit and to reach the help or the
	// file without the shortcuts.
	if a.menu == nil {
		a.menu = native.StatusBar{}
	}

	a.installMenu()

	// A launch of the running app (Alfred, `open -a`, the Finder) shows
	// the panel like the shortcut does.
	native.HandleReopen()
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
		a.showLocked(tab)
		return
	}

	// The other shortcut with the panel up switches tab without hiding
	// it; the page starts clean on the new tab.
	a.tab = tab
	a.window.Emit("shown", tab)
}

// showLocked brings the hidden panel up on tab: placed on its screen,
// shown, given the keyboard, and the page told to start clean. Every way
// in (the shortcuts, the menu, a launch from outside) ends here, so that
// order lives in one place. The caller holds a.mu and
// has checked that the panel is hidden and that no dialog is open.
func (a *App) showLocked(tab string) {
	a.placeWindow()
	a.window.Show()
	a.window.Activate()
	a.visible = true
	a.tab = tab

	// The page resets its selection, search and filter on every
	// appearance.
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
// nothing: the page's blur fires as the dialog takes the focus. Before
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
// focus problems in the webview can be traced without an inspector.
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
