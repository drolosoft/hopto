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

// These tests add, hide and refetch apps from .app bundles with bundle
// ids, a macOS fixture, so they run everywhere but Windows. The link and
// category tests stay in app_edit_test.go.

// TestAppCRUDAndHiding adds, edits and deletes an app by hand, and hides
// and brings back a found one.
func TestAppCRUDAndHiding(t *testing.T) {
	app, _, _ := newTestApp(t)
	bundle := fakeBundle(t, app, "Alpha", "com.example.alpha")

	// A second, unrelated bundle: hiding must be exercised on an app that
	// stays discovered, not on "app-alpha", which TestAHandAddedAppHides
	// ItsDiscoveredTwin covers as the hand-added "alpha"'s twin (Items
	// leaves a twin out entirely, so it cannot be hidden there).
	fakeBundle(t, app, "Beta", "com.example.beta")

	result, err := app.AddApp(AppInput{Path: bundle, Category: "tools"})
	if err != nil || result.ID != "alpha" || len(result.Problems) != 0 {
		t.Fatalf("add: %+v %v", result, err)
	}

	items := app.Items(tabApps)
	wrongItem := items[0].Name != "Alpha" || items[0].Missing ||
		!strings.HasPrefix(items[0].IconURL, "/user-icons/alpha.png?v=")
	if wrongItem {
		t.Errorf("added = %+v", items[0])
	}

	result, _ = app.AddApp(
		AppInput{Path: "/tmp/x.app", Name: "Bad", Category: "tools"},
	)
	if result.Problems["path"] != "app.path" {
		t.Errorf("bad path: %+v", result)
	}

	result, _ = app.AddApp(AppInput{Name: "Nothing", Category: "tools"})
	if result.Problems["path"] != "app.target" {
		t.Errorf("no target: %+v", result)
	}

	updated, err := app.UpdateApp("alpha", AppInput{
		Path:        bundle,
		Name:        "Alpha One",
		Description: "Renamed",
		Category:    "tools",
	})
	if err != nil || updated.ID != "alpha" {
		t.Errorf("update: %+v %v", updated, err)
	}

	if err := app.HideApp("alpha"); !errors.Is(err, errNotDiscovered) {
		t.Errorf("hiding a library app: %v", err)
	}

	if err := app.HideApp("app-beta"); err != nil {
		t.Fatal(err)
	}

	if err := app.HideApp("app-beta"); err != nil {
		t.Errorf("hiding twice must be idempotent: %v", err)
	}

	hidden := false
	for _, item := range app.Items(tabApps) {
		if item.ID == "app-beta" && item.Hidden {
			hidden = true
		}
	}

	if !hidden {
		t.Error("app-beta is not hidden")
	}

	if err := app.UnhideApp("app-beta"); err != nil {
		t.Fatal(err)
	}

	if len(app.library.Snapshot().Hidden) != 0 {
		t.Error("still hidden")
	}

	deleteErr := app.DeleteApp("alpha")
	if deleteErr != nil || len(app.library.Snapshot().Apps) != 0 {
		t.Errorf("delete: %v", deleteErr)
	}
}

// Adopting a hidden discovered app by hand must close its "Hidden" chip,
// not leave it pointing at nothing: AddApp drops the twin's [[hidden]]
// row in the same commit that adds the hand-added entry.
func TestAddAppDropsAHiddenTwinsHiddenRow(t *testing.T) {
	app, _, _ := newTestApp(t)
	bundle := fakeBundle(t, app, "Alpha", "com.example.alpha")

	if err := app.HideApp("app-alpha"); err != nil {
		t.Fatal(err)
	}

	result, err := app.AddApp(AppInput{Path: bundle, Category: "tools"})
	if err != nil || result.ID != "alpha" {
		t.Fatalf("add: %+v %v", result, err)
	}

	for _, category := range app.Categories(tabApps) {
		if category.ID == hiddenChip {
			t.Error("hidden chip survives the twin's adoption")
		}
	}

	if hidden := app.library.Snapshot().Hidden; len(hidden) != 0 {
		t.Errorf("the discovered twin's hidden row was not dropped: %+v", hidden)
	}
}

// HideApp on the discovered id of an app already adopted by hand is
// unreachable from the UI (a twin never reaches the grid under that id):
// calling it directly is refused rather than recreating the dead row
// AddApp just learned to drop.
func TestHideAppRefusesAnAlreadyAdoptedTwin(t *testing.T) {
	app, _, _ := newTestApp(t)
	bundle := fakeBundle(t, app, "Alpha", "com.example.alpha")

	addApp(t, app, library.AppEntry{
		ID: "alpha", Name: "Alpha", Path: bundle, Category: "tools",
	})

	if err := app.HideApp("app-alpha"); !errors.Is(err, errAlreadyAdopted) {
		t.Errorf("hiding an adopted twin: %v", err)
	}

	if hidden := app.library.Snapshot().Hidden; len(hidden) != 0 {
		t.Errorf("an adopted twin gained a hidden row: %+v", hidden)
	}
}

// An app named like an existing link must not take its id: the two would
// collide on icons/<id>.png and confuse CopyTarget, RevealInFinder and
// iconURL, none of which know which tab an id came from.
func TestAddAppAvoidsALinkID(t *testing.T) {
	app, _, _ := newTestApp(t)
	bundle := fakeBundle(t, app, "GitHub", "com.example.github")

	result, err := app.AddApp(AppInput{Path: bundle, Category: "tools"})
	if err != nil || result.ID != "github-2" || len(result.Problems) != 0 {
		t.Fatalf("add: %+v %v", result, err)
	}
}

// AddApp refuses an app that an entry added by hand already opens, by
// path or by bundle id, even when the page's snapshot said nothing (a
// second editor or a hand edit got there first). A found app is still
// adopted: that is how it gets a category, and the other tests of this
// file pin it.
func TestAddAppRefusesAHandAddedTwin(t *testing.T) {
	app, _, _ := newTestApp(t)
	bundle := fakeBundle(t, app, "Alpha", "com.example.alpha")
	addApp(t, app, library.AppEntry{
		ID:       "alpha",
		Name:     "Alpha",
		Path:     bundle,
		BundleID: "com.example.alpha",
		Category: "tools",
	})

	elsewhere := filepath.Join(app.home, "Applications", "Alpha Copy.app")
	cases := []struct {
		name  string
		input AppInput
	}{
		{
			"same path",
			AppInput{Path: bundle, Name: "Again", Category: "tools"},
		},
		{
			"same bundle id",
			AppInput{
				Path:     elsewhere,
				BundleID: "com.example.alpha",
				Name:     "Copy",
				Category: "tools",
			},
		},
	}

	for _, test := range cases {
		result, err := app.AddApp(test.input)
		wrong := err != nil || result.ID != "" ||
			result.Problems == nil || result.Duplicate == nil ||
			result.Duplicate.ID != "alpha"
		if wrong {
			t.Errorf("%s: %+v %v", test.name, result, err)
		}
	}

	if apps := app.library.Snapshot().Apps; len(apps) != 1 {
		t.Errorf("a twin was stored: %+v", apps)
	}
}

// TestRefetchIcon fetches a link's icon again, reports an unreachable
// host, and refuses a found app, whose icon comes from its bundle.
func TestRefetchIcon(t *testing.T) {
	app, _, _ := newTestApp(t)
	allowPrivate(t, app)
	server := iconAndPage(t)
	bundle := fakeBundle(t, app, "Alpha", "com.example.alpha")

	mine := LinkInput{URL: server.URL + "/page", Name: "Mine", Category: "dev"}
	if _, err := app.AddLink(mine); err != nil {
		t.Fatal(err)
	}
	app.background.Wait()

	url, err := app.RefetchIcon(tabLinks, "mine")
	if err != nil || !strings.HasPrefix(url, "/user-icons/mine.png?v=") {
		t.Errorf("link: %q %v", url, err)
	}

	nowhere := LinkInput{
		URL: "https://nowhere.invalid", Name: "Nowhere", Category: "dev",
	}
	if _, err := app.AddLink(nowhere); err != nil {
		t.Fatal(err)
	}
	app.background.Wait()

	_, err = app.RefetchIcon(tabLinks, "nowhere")
	if !errors.Is(err, errNoIconFound) {
		t.Errorf("unreachable host: %v", err)
	}

	_, err = app.AddApp(AppInput{Path: bundle, Category: "tools"})
	if err != nil {
		t.Fatal(err)
	}

	url, err = app.RefetchIcon(tabApps, "alpha")
	if err != nil || !strings.HasPrefix(url, "/user-icons/alpha.png?v=") {
		t.Errorf("app: %q %v", url, err)
	}

	alphaIcon := filepath.Join(app.iconsDir(), "alpha.png")
	if _, err := os.Stat(alphaIcon); err != nil {
		t.Error("the app icon was not written")
	}

	_, err = app.RefetchIcon(tabApps, "app-alpha")
	if !errors.Is(err, errLiveIcon) {
		t.Errorf("discovered: %v", err)
	}
}

// UpdateApp answers problems and unknown ids the same way after the move
// into Apply.
func TestUpdateAppChecksInsideApply(t *testing.T) {
	app, _, _ := newTestApp(t)
	bundle := fakeBundle(t, app, "Gamma", "com.example.gamma")

	added, err := app.AddApp(
		AppInput{Path: bundle, Name: "Gamma", Category: "tools"},
	)
	if err != nil || added.ID == "" {
		t.Fatalf("add: %+v %v", added, err)
	}

	result, err := app.UpdateApp(
		added.ID, AppInput{Path: bundle, Name: "", Category: "nope"},
	)
	wrong := err != nil || result.ID != "" ||
		result.Problems["name"] != "name.required" ||
		result.Problems["category"] != "category.unknown"
	if wrong {
		t.Errorf("problems: %+v %v", result, err)
	}

	result, err = app.UpdateApp(
		added.ID, AppInput{Path: bundle, Name: "Gamma 2", Category: "tools"},
	)
	if err != nil || result.ID != added.ID {
		t.Errorf("update: %+v %v", result, err)
	}

	_, err = app.UpdateApp("nope", AppInput{Name: "X", Category: "tools"})
	if !errors.Is(err, errUnknownItem) {
		t.Errorf("unknown: %v", err)
	}
}
