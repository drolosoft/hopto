package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/drolosoft/hopto/internal/library"
)

// fakeWindow records what App asks of the window.
type fakeWindow struct {
	mu        sync.Mutex
	calls     []string
	clipboard string

	// What the fake open panel answers: a path, "" for cancel, or an
	// error.
	pickPath string
	pickErr  error

	// Where the last Center asked for the window: the mode and the
	// display screenChoice picked.
	centerMode    string
	centerDisplay uint32
}

// record appends one call under the lock; the calls come from several
// goroutines in the race tests.
func (w *fakeWindow) record(call string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.calls = append(w.calls, call)
}

// Show records "show".
func (w *fakeWindow) Show() { w.record("show") }

// Hide records "hide".
func (w *fakeWindow) Hide() { w.record("hide") }

// Activate records "activate".
func (w *fakeWindow) Activate() { w.record("activate") }

// Emit records "emit:<name>:<data>".
func (w *fakeWindow) Emit(name string, data any) {
	w.record(fmt.Sprintf("emit:%s:%v", name, data))
}

// Center records "center" and remembers where it was asked to go.
func (w *fakeWindow) Center(mode string, display uint32) {
	w.mu.Lock()
	w.centerMode = mode
	w.centerDisplay = display
	w.mu.Unlock()

	w.record("center")
}

// SetClipboard keeps the text for the test to read.
func (w *fakeWindow) SetClipboard(text string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.clipboard = text

	return nil
}

// SetAlwaysOnTop records "ontop:<on>".
func (w *fakeWindow) SetAlwaysOnTop(on bool) {
	w.record(fmt.Sprintf("ontop:%v", on))
}

// PickFile records "pick:<folder>" and answers what the test armed.
func (w *fakeWindow) PickFile(directory string) (string, error) {
	w.record("pick:" + directory)

	w.mu.Lock()
	defer w.mu.Unlock()

	return w.pickPath, w.pickErr
}

// Quit records "quit".
func (w *fakeWindow) Quit() { w.record("quit") }

// fakeMenu records what App puts in the menu bar item.
type fakeMenu struct {
	mu      sync.Mutex
	items   []menuItem
	checked map[int]bool
}

// Install keeps the entries and their ticks, as the real item draws them.
func (m *fakeMenu) Install(items []menuItem) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = items
	m.checked = map[int]bool{}

	for _, item := range items {
		m.checked[item.Tag] = item.Checked
	}
}

// SetChecked changes one tick.
func (m *fakeMenu) SetChecked(tag int, on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.checked[tag] = on
}

// isChecked reads one tick under the lock.
func (m *fakeMenu) isChecked(tag int) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.checked[tag]
}

// joined returns the calls as one string for assertions.
func (w *fakeWindow) joined() string {
	w.mu.Lock()
	defer w.mu.Unlock()

	return strings.Join(w.calls, " ")
}

// fakeOpen stands in for /usr/bin/open.
type fakeOpen struct {
	calls [][]string
	err   error
}

// run records the arguments and fails when the test armed an error.
func (o *fakeOpen) run(args ...string) error {
	o.calls = append(o.calls, args)

	return o.err
}

// newTestApp builds an App over a temp home, with discovery pointed at
// folders inside it and English texts.
func newTestApp(t *testing.T) (*App, *fakeWindow, *fakeOpen) {
	t.Helper()

	home := t.TempDir()
	win := &fakeWindow{}
	open := &fakeOpen{}

	app := newApp(home, dataDir(home), win, open.run, languageEnglish)
	app.appRoots = []string{filepath.Join(home, "Applications")}
	app.edgeDir = filepath.Join(home, "Applications", "Edge Apps.localized")
	app.offline = true

	// The real screen readers wait on the main thread, which go test
	// never runs: no screen under the window, and one display attached.
	app.windowDisplay = func() uint32 { return 0 }
	app.attachedDisplays = func() []uint32 { return []uint32{1} }

	t.Cleanup(app.background.Wait)

	return app, win, open
}

// fakeBundle drops a minimal .app under ~/Applications with the test icns.
func fakeBundle(t *testing.T, app *App, name, bundleID string) string {
	t.Helper()

	bundle := filepath.Join(app.home, "Applications", name+".app")
	resources := filepath.Join(bundle, "Contents", "Resources")
	if err := os.MkdirAll(resources, 0o755); err != nil {
		t.Fatal(err)
	}

	plist := `<?xml version="1.0" encoding="UTF-8"?>` +
		`<plist version="1.0"><dict>` +
		`<key>CFBundleIdentifier</key><string>` + bundleID + `</string>` +
		`<key>CFBundleIconFile</key><string>App</string></dict></plist>`
	infoPath := filepath.Join(bundle, "Contents", "Info.plist")
	if err := os.WriteFile(infoPath, []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}

	icns, err := os.ReadFile("internal/icons/testdata/edge-app.icns")
	if err != nil {
		t.Fatal(err)
	}

	icnsPath := filepath.Join(resources, "App.icns")
	if err := os.WriteFile(icnsPath, icns, 0o644); err != nil {
		t.Fatal(err)
	}

	return bundle
}

// addApp puts a hand-added app in the library.
func addApp(t *testing.T, app *App, entry library.AppEntry) {
	t.Helper()

	err := app.library.Apply(func(lib *library.Library) error {
		lib.Apps = append(lib.Apps, entry)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// The five cases of the two shortcuts.
func TestToggle(t *testing.T) {
	cases := []struct {
		name        string
		visible     bool
		tab         string
		press       string
		wantVisible bool
		wantCalls   string
	}{
		{
			"hidden, apps shows apps",
			false, tabApps, tabApps, true,
			"center show activate emit:shown:apps",
		},
		{
			"hidden, links shows links",
			false, tabApps, tabLinks, true,
			"center show activate emit:shown:links",
		},
		{
			"apps showing, apps hides",
			true, tabApps, tabApps, false, "hide",
		},
		{
			"links showing, links hides",
			true, tabLinks, tabLinks, false, "hide",
		},
		{
			"apps showing, links switches",
			true, tabApps, tabLinks, true, "emit:shown:links",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, win, _ := newTestApp(t)
			app.visible = tc.visible
			app.tab = tc.tab

			app.toggle(tc.press)

			if app.visible != tc.wantVisible || app.tab != tc.press {
				t.Errorf("visible %v tab %s", app.visible, app.tab)
			}

			if got := win.joined(); got != tc.wantCalls {
				t.Errorf("calls %q, want %q", got, tc.wantCalls)
			}
		})
	}
}

// The seed is what a fresh install lists; every field the page reads is
// filled and no collection is nil.
func TestItemsListsTheSeedLinks(t *testing.T) {
	app, _, _ := newTestApp(t)

	links := app.Items(tabLinks)
	if len(links) != 6 {
		t.Fatalf("got %d links", len(links))
	}

	github := links[0]
	wrongGithub := github.ID != "github" ||
		github.Key != "links:github" ||
		github.Kind != kindLink ||
		github.Source != sourceLibrary ||
		github.Host != "github.com"
	if wrongGithub {
		t.Errorf("github = %+v", github)
	}

	if github.Keywords == nil || github.IconURL != "" {
		t.Errorf("keywords %v icon %q", github.Keywords, github.IconURL)
	}

	categories := app.Categories(tabLinks)
	wrongCategories := len(categories) != 2 ||
		categories[0].ID != "dev" || categories[0].Virtual
	if wrongCategories {
		t.Errorf("categories = %+v", categories)
	}
}

// Discovered apps join the hand-added ones, with virtual categories and
// icons served from their bundles; hidden ones are flagged, not dropped.
func TestItemsListsDiscoveredApps(t *testing.T) {
	app, _, _ := newTestApp(t)
	fakeBundle(t, app, "Alpha", "com.example.alpha")
	addApp(t, app, library.AppEntry{
		ID: "mine", Name: "Mine",
		BundleID: "com.example.mine", Category: "tools",
	})

	err := app.library.Apply(func(lib *library.Library) error {
		lib.Hidden = append(lib.Hidden, library.Hidden{ID: "app-alpha"})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	apps := app.Items(tabApps)
	wrongApps := len(apps) != 2 ||
		apps[0].ID != "mine" || apps[1].ID != "app-alpha"
	if wrongApps {
		t.Fatalf("apps = %+v", apps)
	}

	alpha := apps[1]
	wrongAlpha := alpha.Source != "applications" ||
		alpha.Category != "applications" ||
		!alpha.Hidden || alpha.Key != "apps:app-alpha"
	if wrongAlpha {
		t.Errorf("alpha = %+v", alpha)
	}

	if !strings.HasPrefix(alpha.IconURL, "/user-icons/app-alpha.png?v=") {
		t.Errorf("icon = %q", alpha.IconURL)
	}

	if apps[0].Missing || apps[0].IconURL != "" {
		t.Errorf("mine = %+v", apps[0])
	}

	// app-alpha is hidden, so the "hidden" chip closes the row.
	categories := app.Categories(tabApps)
	wrong := len(categories) != 3 ||
		categories[1].ID != "applications" || !categories[1].Virtual ||
		categories[2].ID != hiddenChip || !categories[2].Virtual
	if wrong {
		t.Errorf("categories = %+v", categories)
	}

	if _, ok := app.iconSource("app-alpha"); !ok {
		t.Error("the handler cannot resolve the discovered icon")
	}
}

// The hidden chip shows only while a present app is hidden, and no user
// category can take its id.
func TestHiddenChipComesAndGoes(t *testing.T) {
	app, _, _ := newTestApp(t)
	fakeBundle(t, app, "Alpha", "com.example.alpha")

	hasHidden := func() bool {
		for _, category := range app.Categories(tabApps) {
			if category.ID == hiddenChip {
				return true
			}
		}

		return false
	}

	if hasHidden() {
		t.Fatal("hidden chip with nothing hidden")
	}

	if err := app.HideApp("app-alpha"); err != nil || !hasHidden() {
		t.Fatalf("after hiding: %v", err)
	}

	if err := app.UnhideApp("app-alpha"); err != nil || hasHidden() {
		t.Fatalf("after unhiding: %v", err)
	}

	view, err := app.AddCategory(tabApps, "Hidden")
	if err != nil || view.ID != "hidden-2" {
		t.Errorf("category = %+v %v", view, err)
	}
}

// A hand-added app whose path is gone is listed as missing.
func TestItemsFlagsMissingApps(t *testing.T) {
	app, _, _ := newTestApp(t)
	addApp(t, app, library.AppEntry{
		ID: "gone", Name: "Gone",
		Path:     filepath.Join(app.home, "Applications", "Gone.app"),
		Category: "tools",
	})

	apps := app.Items(tabApps)
	if len(apps) != 1 || !apps[0].Missing {
		t.Errorf("apps = %+v", apps)
	}
}

// TestLaunch refuses an unknown id and a missing bundle without calling
// open, keeps the panel up when open fails, opens by bundle id, counts
// and hides on success, and finds a found app with no Items call first.
func TestLaunch(t *testing.T) {
	t.Run("unknown id opens nothing", func(t *testing.T) {
		app, _, open := newTestApp(t)

		err := app.Launch("nope")
		if !errors.Is(err, errUnknownItem) || len(open.calls) != 0 {
			t.Errorf("err %v calls %v", err, open.calls)
		}
	})

	t.Run("missing path is reported", func(t *testing.T) {
		app, _, open := newTestApp(t)
		addApp(t, app, library.AppEntry{
			ID: "gone", Name: "Gone",
			Path:     filepath.Join(app.home, "Applications", "Gone.app"),
			Category: "tools",
		})

		err := app.Launch("gone")
		if !errors.Is(err, errNotInstalled) || len(open.calls) != 0 {
			t.Errorf("err %v calls %v", err, open.calls)
		}
	})

	t.Run("open failure keeps the panel", func(t *testing.T) {
		app, win, open := newTestApp(t)
		open.err = errors.New("boom")
		bundle := fakeBundle(t, app, "Alpha", "com.example.alpha")
		app.Items(tabApps)

		if err := app.Launch("app-alpha"); err == nil || win.joined() != "" {
			t.Errorf("err %v calls %q", err, win.joined())
		}

		if len(open.calls) != 1 || open.calls[0][0] != bundle {
			t.Errorf("open calls %v", open.calls)
		}
	})

	t.Run("success opens by bundle id, counts and hides", func(t *testing.T) {
		app, win, open := newTestApp(t)
		addApp(t, app, library.AppEntry{
			ID: "mine", Name: "Mine",
			BundleID: "com.example.mine", Category: "tools",
		})
		app.visible = true

		if err := app.Launch("mine"); err != nil {
			t.Fatal(err)
		}

		opened := len(open.calls) != 1 ||
			strings.Join(open.calls[0], " ") != "-b com.example.mine"
		if opened {
			t.Errorf("open calls %v", open.calls)
		}

		wrong := win.joined() != "hide" || app.visible ||
			app.Usage().Opens["apps:mine"] != 1
		if wrong {
			t.Errorf(
				"calls %q visible %v opens %v",
				win.joined(), app.visible, app.Usage().Opens,
			)
		}
	})

	t.Run(
		"discovered app is found without a prior Items call",
		func(t *testing.T) {
			app, _, open := newTestApp(t)
			bundle := fakeBundle(t, app, "Alpha", "com.example.alpha")

			err := app.Launch("app-alpha")
			if err != nil || open.calls[0][0] != bundle {
				t.Errorf("err %v calls %v", err, open.calls)
			}
		},
	)
}

// Only library ids reach `open`: a URL, a scheme, anything else is unknown.
func TestOpenLinkNeverOpensAnythingButLibraryURLs(t *testing.T) {
	app, win, open := newTestApp(t)

	ids := []string{"javascript:alert(1)", "https://evil.test", "", "../x"}
	for _, id := range ids {
		if err := app.OpenLink(id); !errors.Is(err, errUnknownItem) {
			t.Errorf("%q: err = %v", id, err)
		}
	}

	if len(open.calls) != 0 {
		t.Fatalf("open was called: %v", open.calls)
	}

	if err := app.OpenLink("github"); err != nil {
		t.Fatal(err)
	}

	wrong := strings.Join(open.calls[0], " ") != "https://github.com" ||
		win.joined() != "hide" ||
		app.Usage().Opens["links:github"] != 1
	if wrong {
		t.Errorf("calls %v window %q", open.calls, win.joined())
	}
}

// ⌘↩ needs a secondary browser in the settings, then opens with -b.
func TestOpenLinkWith(t *testing.T) {
	app, _, open := newTestApp(t)

	err := app.OpenLinkWith("github")
	if !errors.Is(err, errNoSecondaryBrowser) || len(open.calls) != 0 {
		t.Fatalf("err %v calls %v", err, open.calls)
	}

	err = app.library.Apply(func(lib *library.Library) error {
		lib.Settings.SecondaryBrowser = "com.apple.Safari"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := app.OpenLinkWith("github"); err != nil {
		t.Fatal(err)
	}

	wantCall := "-b com.apple.Safari https://github.com"
	if strings.Join(open.calls[0], " ") != wantCall {
		t.Errorf("calls %v", open.calls)
	}
}

// TestCopyAndReveal copies a link's URL, an app's path or its bundle id,
// reveals apps but not links in the Finder, and reveals and edits
// library.toml.
func TestCopyAndReveal(t *testing.T) {
	app, win, open := newTestApp(t)
	bundle := fakeBundle(t, app, "Alpha", "com.example.alpha")
	addApp(t, app, library.AppEntry{
		ID: "mine", Name: "Mine",
		BundleID: "com.example.mine", Category: "tools",
	})
	app.Items(tabApps)

	linkErr := app.CopyTarget("github")
	if linkErr != nil || win.clipboard != "https://github.com" {
		t.Errorf("link: err %v clipboard %q", linkErr, win.clipboard)
	}

	appErr := app.CopyTarget("app-alpha")
	if appErr != nil || win.clipboard != bundle {
		t.Errorf("app: err %v clipboard %q", appErr, win.clipboard)
	}

	bundleErr := app.CopyTarget("mine")
	if bundleErr != nil || win.clipboard != "com.example.mine" {
		t.Errorf("bundle id: err %v clipboard %q", bundleErr, win.clipboard)
	}

	revealErr := app.RevealInFinder("app-alpha")
	if revealErr != nil || strings.Join(open.calls[0], " ") != "-R "+bundle {
		t.Errorf("reveal: err %v calls %v", revealErr, open.calls)
	}

	if err := app.RevealInFinder("github"); !errors.Is(err, errNoPath) {
		t.Errorf("reveal link: err %v", err)
	}

	libErr := app.RevealLibrary()
	wantReveal := "-R " + app.library.Path()
	if libErr != nil || strings.Join(open.calls[1], " ") != wantReveal {
		t.Errorf("reveal library: %v %v", libErr, open.calls)
	}

	editErr := app.EditLibrary()
	wantEdit := "-t " + app.library.Path()
	if editErr != nil || strings.Join(open.calls[2], " ") != wantEdit {
		t.Errorf("edit library: %v %v", editErr, open.calls)
	}
}

// A favourite key must name something that exists.
func TestToggleFavoriteRequiresAnExistingKey(t *testing.T) {
	app, _, _ := newTestApp(t)

	keys := []string{"links:nope", "apps:github", "github", "links:../x"}
	for _, key := range keys {
		if _, err := app.ToggleFavorite(key); !errors.Is(err, errUnknownItem) {
			t.Errorf("%q: err = %v", key, err)
		}
	}

	if on, err := app.ToggleFavorite("links:github"); err != nil || !on {
		t.Errorf("on %v err %v", on, err)
	}

	favorites := app.Usage().Favorites
	if len(favorites) != 1 || favorites[0] != "links:github" {
		t.Errorf("favorites = %v", favorites)
	}
}

// Settings resolve "auto" for the page; the status reports a broken file
// with its line while the seed stays on screen.
func TestSettingsAndStatus(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.language = languageSpanish

	settings := app.Settings()
	wrongSettings := settings.Language != languageSpanish ||
		settings.HotkeyApps != defaultAppsHotkey ||
		settings.IconServices == nil
	if wrongSettings {
		t.Errorf("settings = %+v", settings)
	}

	if status := app.LibraryStatus(); status.ReadOnly || status.Error != "" {
		t.Errorf("status = %+v", status)
	}

	broken := []byte("version = 1\n\n[[links]\nbroken\n")
	if err := os.WriteFile(app.library.Path(), broken, 0o600); err != nil {
		t.Fatal(err)
	}

	if len(app.Items(tabLinks)) != 6 {
		t.Error("the last good library must stay on screen")
	}

	// BurntSushi reports the unterminated "[[links]" at the line after the
	// one holding it, once it reaches the newline it did not expect there.
	status := app.LibraryStatus()
	wrongStatus := !status.ReadOnly || status.Line != 4 ||
		!strings.HasSuffix(status.Path, "library.toml")
	if wrongStatus {
		t.Errorf("status = %+v", status)
	}
}

// Debug lines are cut and cleaned before they reach the log.
func TestDebugIsSanitised(t *testing.T) {
	got := debugLine(strings.Repeat("a", 600) + "\x1b[31m\n")
	if len([]rune(got)) != maxDebugRunes || strings.ContainsAny(got, "\x1b\n") {
		t.Errorf("debugLine = %q (%d runes)", got, len([]rune(got)))
	}
}

// systemBundle drops a minimal .app under the fake /System/Applications
// of the test's home.
func systemBundle(t *testing.T, app *App, name, bundleID string) {
	t.Helper()

	bundle := filepath.Join(app.systemApps, name+".app")
	contents := filepath.Join(bundle, "Contents")
	if err := os.MkdirAll(contents, 0o755); err != nil {
		t.Fatal(err)
	}

	plist := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0">` +
		`<dict><key>CFBundleIdentifier</key><string>` + bundleID +
		`</string></dict></plist>`
	infoPath := filepath.Join(contents, "Info.plist")
	if err := os.WriteFile(infoPath, []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Supuesto 1: the system's apps come out of the same scan, marked search
// only, once even when a copy also sits in ~/Applications, and they do
// not make the "applications" chip appear on their own.
func TestSystemAppsAreSearchOnly(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.systemApps = filepath.Join(app.home, "System", "Applications")
	app.appRoots = append(app.appRoots, app.systemApps)

	systemBundle(t, app, "Calculator", "com.example.calculator")

	for _, category := range app.Categories(tabApps) {
		if category.ID == "applications" {
			t.Errorf("a search-only app made its chip appear")
		}
	}

	fakeBundle(t, app, "Alpha", "com.example.alpha")
	systemBundle(t, app, "Alpha", "com.example.alpha")

	byID := map[string]ItemView{}
	for _, view := range app.Items(tabApps) {
		byID[view.ID] = view
	}

	if len(byID) != 2 {
		t.Fatalf("items = %+v", byID)
	}

	calculator := byID["app-calculator"]
	alpha := byID["app-alpha"]

	if !calculator.SearchOnly || alpha.SearchOnly {
		t.Errorf("calculator = %+v, alpha = %+v", calculator, alpha)
	}

	appsRoot := filepath.Join(app.home, "Applications")
	if !strings.HasPrefix(alpha.Path, appsRoot) {
		t.Errorf("the ~/Applications copy should win: %s", alpha.Path)
	}

	// A hidden app must still reach the page under the Hidden chip, even
	// when it is also search only: SearchOnly hides it from the ordinary
	// grid and chips, but Hidden is the one place it has to come back
	// from, or ⌘⌫ would have no way to unhide it once it is gone.
	if err := app.HideApp(calculator.ID); err != nil {
		t.Fatal(err)
	}

	hasHiddenChip := false
	for _, category := range app.Categories(tabApps) {
		hasHiddenChip = hasHiddenChip || category.ID == hiddenChip
	}
	if !hasHiddenChip {
		t.Error("a hidden search-only app must open the Hidden chip")
	}

	for _, view := range app.Items(tabApps) {
		if view.ID != calculator.ID {
			continue
		}

		if !view.Hidden || !view.SearchOnly {
			t.Errorf("hidden calculator = %+v", view)
		}
	}
}

// An app found on disk and also added by hand (to give it a category)
// is listed once, as the hand-added entry, matched by path or bundle id.
func TestAHandAddedAppHidesItsDiscoveredTwin(t *testing.T) {
	app, _, _ := newTestApp(t)
	alpha := fakeBundle(t, app, "Alpha", "com.example.alpha")
	fakeBundle(t, app, "Beta", "com.example.beta")

	addApp(t, app, library.AppEntry{
		ID: "alpha", Name: "Alpha", Path: alpha, Category: "tools",
	})
	addApp(t, app, library.AppEntry{
		ID: "beta", Name: "Beta", BundleID: "com.example.beta",
		Category: "tools",
	})

	ids := []string{}
	for _, view := range app.Items(tabApps) {
		ids = append(ids, view.ID)
	}

	if strings.Join(ids, " ") != "alpha beta" {
		t.Errorf("ids = %v", ids)
	}
}
