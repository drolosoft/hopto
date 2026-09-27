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
}

func (w *fakeWindow) record(call string) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.calls = append(w.calls, call)
}

func (w *fakeWindow) Show()     { w.record("show") }
func (w *fakeWindow) Hide()     { w.record("hide") }
func (w *fakeWindow) Center()   { w.record("center") }
func (w *fakeWindow) Activate() { w.record("activate") }

func (w *fakeWindow) Emit(name string, data any) {
	w.record(fmt.Sprintf("emit:%s:%v", name, data))
}

func (w *fakeWindow) SetClipboard(text string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.clipboard = text

	return nil
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

	t.Cleanup(app.background.Wait)

	return app, win, open
}

// fakeBundle drops a minimal .app under ~/Applications with the test icns.
func fakeBundle(t *testing.T, app *App, name, bundleID string) string {
	t.Helper()

	bundle := filepath.Join(app.home, "Applications", name+".app")
	if err := os.MkdirAll(filepath.Join(bundle, "Contents", "Resources"), 0o755); err != nil {
		t.Fatal(err)
	}

	plist := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict>` +
		`<key>CFBundleIdentifier</key><string>` + bundleID + `</string>` +
		`<key>CFBundleIconFile</key><string>App</string></dict></plist>`
	if err := os.WriteFile(filepath.Join(bundle, "Contents", "Info.plist"), []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}

	icns, err := os.ReadFile("internal/icons/testdata/edge-app.icns")
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(bundle, "Contents", "Resources", "App.icns"), icns, 0o644); err != nil {
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
		{"hidden, apps shows apps", false, tabApps, tabApps, true, "center show activate emit:shown:apps"},
		{"hidden, links shows links", false, tabApps, tabLinks, true, "center show activate emit:shown:links"},
		{"apps showing, apps hides", true, tabApps, tabApps, false, "hide"},
		{"links showing, links hides", true, tabLinks, tabLinks, false, "hide"},
		{"apps showing, links switches", true, tabApps, tabLinks, true, "emit:shown:links"},
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
	if github.ID != "github" || github.Key != "links:github" || github.Kind != kindLink || github.Source != sourceLibrary || github.Host != "github.com" {
		t.Errorf("github = %+v", github)
	}

	if github.Keywords == nil || github.IconURL != "" {
		t.Errorf("keywords %v icon %q", github.Keywords, github.IconURL)
	}

	categories := app.Categories(tabLinks)
	if len(categories) != 2 || categories[0].ID != "dev" || categories[0].Virtual {
		t.Errorf("categories = %+v", categories)
	}
}

// Discovered apps join the hand-added ones, with virtual categories and
// icons served from their bundles; hidden ones are flagged, not dropped.
func TestItemsListsDiscoveredApps(t *testing.T) {
	app, _, _ := newTestApp(t)
	fakeBundle(t, app, "Alpha", "com.example.alpha")
	addApp(t, app, library.AppEntry{ID: "mine", Name: "Mine", BundleID: "com.example.mine", Category: "tools"})

	err := app.library.Apply(func(lib *library.Library) error {
		lib.Hidden = append(lib.Hidden, library.Hidden{ID: "app-alpha"})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	apps := app.Items(tabApps)
	if len(apps) != 2 || apps[0].ID != "mine" || apps[1].ID != "app-alpha" {
		t.Fatalf("apps = %+v", apps)
	}

	alpha := apps[1]
	if alpha.Source != "applications" || alpha.Category != "applications" || !alpha.Hidden || alpha.Key != "apps:app-alpha" {
		t.Errorf("alpha = %+v", alpha)
	}

	if !strings.HasPrefix(alpha.IconURL, "/user-icons/app-alpha.png?v=") {
		t.Errorf("icon = %q", alpha.IconURL)
	}

	if apps[0].Missing || apps[0].IconURL != "" {
		t.Errorf("mine = %+v", apps[0])
	}

	categories := app.Categories(tabApps)
	if len(categories) != 2 || categories[1].ID != "applications" || !categories[1].Virtual {
		t.Errorf("categories = %+v", categories)
	}

	if _, ok := app.iconSource("app-alpha"); !ok {
		t.Error("the handler cannot resolve the discovered icon")
	}
}

// A hand-added app whose path is gone is listed as missing.
func TestItemsFlagsMissingApps(t *testing.T) {
	app, _, _ := newTestApp(t)
	addApp(t, app, library.AppEntry{ID: "gone", Name: "Gone", Path: filepath.Join(app.home, "Applications", "Gone.app"), Category: "tools"})

	apps := app.Items(tabApps)
	if len(apps) != 1 || !apps[0].Missing {
		t.Errorf("apps = %+v", apps)
	}
}

func TestLaunch(t *testing.T) {
	t.Run("unknown id opens nothing", func(t *testing.T) {
		app, _, open := newTestApp(t)

		if err := app.Launch("nope"); !errors.Is(err, errUnknownItem) || len(open.calls) != 0 {
			t.Errorf("err %v calls %v", err, open.calls)
		}
	})

	t.Run("missing path is reported", func(t *testing.T) {
		app, _, open := newTestApp(t)
		addApp(t, app, library.AppEntry{ID: "gone", Name: "Gone", Path: filepath.Join(app.home, "Applications", "Gone.app"), Category: "tools"})

		if err := app.Launch("gone"); !errors.Is(err, errNotInstalled) || len(open.calls) != 0 {
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
		addApp(t, app, library.AppEntry{ID: "mine", Name: "Mine", BundleID: "com.example.mine", Category: "tools"})
		app.visible = true

		if err := app.Launch("mine"); err != nil {
			t.Fatal(err)
		}

		if len(open.calls) != 1 || strings.Join(open.calls[0], " ") != "-b com.example.mine" {
			t.Errorf("open calls %v", open.calls)
		}

		if win.joined() != "hide" || app.visible || app.Usage().Opens["apps:mine"] != 1 {
			t.Errorf("calls %q visible %v opens %v", win.joined(), app.visible, app.Usage().Opens)
		}
	})

	t.Run("discovered app is found without a prior Items call", func(t *testing.T) {
		app, _, open := newTestApp(t)
		bundle := fakeBundle(t, app, "Alpha", "com.example.alpha")

		if err := app.Launch("app-alpha"); err != nil || open.calls[0][0] != bundle {
			t.Errorf("err %v calls %v", err, open.calls)
		}
	})
}

// Only library ids reach `open`: a URL, a scheme, anything else is unknown.
func TestOpenLinkNeverOpensAnythingButLibraryURLs(t *testing.T) {
	app, win, open := newTestApp(t)

	for _, id := range []string{"javascript:alert(1)", "https://evil.test", "", "../x"} {
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

	if strings.Join(open.calls[0], " ") != "https://github.com" || win.joined() != "hide" || app.Usage().Opens["links:github"] != 1 {
		t.Errorf("calls %v window %q", open.calls, win.joined())
	}
}

// ⌘↩ needs a secondary browser in the settings, then opens with -b.
func TestOpenLinkWith(t *testing.T) {
	app, _, open := newTestApp(t)

	if err := app.OpenLinkWith("github"); !errors.Is(err, errNoSecondaryBrowser) || len(open.calls) != 0 {
		t.Fatalf("err %v calls %v", err, open.calls)
	}

	err := app.library.Apply(func(lib *library.Library) error {
		lib.Settings.SecondaryBrowser = "com.apple.Safari"
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := app.OpenLinkWith("github"); err != nil {
		t.Fatal(err)
	}

	if strings.Join(open.calls[0], " ") != "-b com.apple.Safari https://github.com" {
		t.Errorf("calls %v", open.calls)
	}
}

func TestCopyAndReveal(t *testing.T) {
	app, win, open := newTestApp(t)
	bundle := fakeBundle(t, app, "Alpha", "com.example.alpha")
	addApp(t, app, library.AppEntry{ID: "mine", Name: "Mine", BundleID: "com.example.mine", Category: "tools"})
	app.Items(tabApps)

	if err := app.CopyTarget("github"); err != nil || win.clipboard != "https://github.com" {
		t.Errorf("link: err %v clipboard %q", err, win.clipboard)
	}

	if err := app.CopyTarget("app-alpha"); err != nil || win.clipboard != bundle {
		t.Errorf("app: err %v clipboard %q", err, win.clipboard)
	}

	if err := app.CopyTarget("mine"); err != nil || win.clipboard != "com.example.mine" {
		t.Errorf("bundle id: err %v clipboard %q", err, win.clipboard)
	}

	if err := app.RevealInFinder("app-alpha"); err != nil || strings.Join(open.calls[0], " ") != "-R "+bundle {
		t.Errorf("reveal: err %v calls %v", err, open.calls)
	}

	if err := app.RevealInFinder("github"); !errors.Is(err, errNoPath) {
		t.Errorf("reveal link: err %v", err)
	}

	if err := app.RevealLibrary(); err != nil || strings.Join(open.calls[1], " ") != "-R "+app.library.Path() {
		t.Errorf("reveal library: %v %v", err, open.calls)
	}

	if err := app.EditLibrary(); err != nil || strings.Join(open.calls[2], " ") != "-t "+app.library.Path() {
		t.Errorf("edit library: %v %v", err, open.calls)
	}
}

// A favourite key must name something that exists.
func TestToggleFavoriteRequiresAnExistingKey(t *testing.T) {
	app, _, _ := newTestApp(t)

	for _, key := range []string{"links:nope", "apps:github", "github", "links:../x"} {
		if _, err := app.ToggleFavorite(key); !errors.Is(err, errUnknownItem) {
			t.Errorf("%q: err = %v", key, err)
		}
	}

	if on, err := app.ToggleFavorite("links:github"); err != nil || !on {
		t.Errorf("on %v err %v", on, err)
	}

	if favorites := app.Usage().Favorites; len(favorites) != 1 || favorites[0] != "links:github" {
		t.Errorf("favorites = %v", favorites)
	}
}

// Settings resolve "auto" for the page; the status reports a broken file
// with its line while the seed stays on screen.
func TestSettingsAndStatus(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.language = languageSpanish

	settings := app.Settings()
	if settings.Language != languageSpanish || settings.HotkeyApps != defaultAppsHotkey || settings.IconServices == nil {
		t.Errorf("settings = %+v", settings)
	}

	if status := app.LibraryStatus(); status.ReadOnly || status.Error != "" {
		t.Errorf("status = %+v", status)
	}

	if err := os.WriteFile(app.library.Path(), []byte("version = 1\n\n[[links]\nbroken\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if len(app.Items(tabLinks)) != 6 {
		t.Error("the last good library must stay on screen")
	}

	// BurntSushi reports the unterminated "[[links]" at the line after the
	// one holding it, once it reaches the newline it did not expect there.
	status := app.LibraryStatus()
	if !status.ReadOnly || status.Line != 4 || !strings.HasSuffix(status.Path, "library.toml") {
		t.Errorf("status = %+v", status)
	}
}

// Debug lines are cut and cleaned before they reach the log.
func TestDebugIsSanitised(t *testing.T) {
	if got := debugLine(strings.Repeat("a", 600) + "\x1b[31m\n"); len([]rune(got)) != maxDebugRunes || strings.ContainsAny(got, "\x1b\n") {
		t.Errorf("debugLine = %q (%d runes)", got, len([]rune(got)))
	}
}
