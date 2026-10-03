//go:build !windows

package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drolosoft/hopto/internal/library"
)

// These tests drop .app bundles under ~/Applications and launch them by
// bundle id, a macOS fixture, so they run everywhere but Windows. The
// engine tests that need no bundle stay in app_test.go.

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

	icns, err := os.ReadFile("../icons/testdata/edge-app.icns")
	if err != nil {
		t.Fatal(err)
	}

	icnsPath := filepath.Join(resources, "App.icns")
	if err := os.WriteFile(icnsPath, icns, 0o644); err != nil {
		t.Fatal(err)
	}

	return bundle
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
