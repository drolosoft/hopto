package main

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/hopto/internal/library"
)

// solidPNG draws a flat square icon, the same shape a real favicon has
// once decoded; borrowed from the icons package's own tests.
func solidPNG(t *testing.T, side int) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	for y := range side {
		for x := range side {
			img.Set(x, y, color.NRGBA{R: 10, G: 120, B: 200, A: 255})
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}

	return out.Bytes()
}

// iconAndPage serves a 64 px PNG and a page that links to it.
func iconAndPage(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc(
		"GET /icon.png",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(solidPNG(t, 64))
		},
	)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Example Page</title>
<meta property="og:site_name" content="Example">
<meta name="description" content="An example site">
<link rel="apple-touch-icon" href="/icon.png"></head></html>`))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server
}

// allowPrivate opts the test app into loopback fetches and turns the
// background fetches on (newTestApp keeps them off).
func allowPrivate(t *testing.T, app *App) {
	t.Helper()

	app.offline = false

	err := app.library.Apply(func(lib *library.Library) error {
		lib.Settings.AllowPrivateIconHosts = true
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// waitForCall polls the fake window for a call, up to a limit.
func waitForCall(t *testing.T, win *fakeWindow, wanted string) {
	t.Helper()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(win.joined(), wanted) {
			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatalf("no %q among %q", wanted, win.joined())
}

func TestAddLink(t *testing.T) {
	app, _, _ := newTestApp(t)

	result, err := app.AddLink(
		LinkInput{URL: "ftp://x", Name: "", Category: "nope"},
	)
	if err != nil {
		t.Fatal(err)
	}

	wrongProblems := result.Problems["url"] != "url.invalid" ||
		result.Problems["name"] != "name.required" ||
		result.Problems["category"] != "category.unknown" ||
		result.ID != ""
	if wrongProblems {
		t.Errorf("problems = %+v", result)
	}

	result, err = app.AddLink(
		LinkInput{URL: "GITHUB.com/", Name: "Again", Category: "dev"},
	)
	wrongDuplicate := err != nil || result.Duplicate == nil ||
		result.Duplicate.ID != "github" || result.ID != ""
	if wrongDuplicate {
		t.Errorf("duplicate: %+v %v", result, err)
	}

	result, err = app.AddLink(LinkInput{
		URL:         "my-site.test/docs",
		Name:        "My Site",
		Description: " Docs ",
		Category:    "docs",
		Keywords:    []string{" api ", ""},
	})
	if err != nil || result.ID != "my-site" || len(result.Problems) != 0 {
		t.Fatalf("add: %+v %v", result, err)
	}

	link, ok := app.findLink("my-site")
	wrongLink := !ok || link.URL != "https://my-site.test/docs" ||
		link.Description != "Docs" || len(link.Keywords) != 1 ||
		link.Keywords[0] != "api"
	if wrongLink {
		t.Errorf("stored = %+v", link)
	}

	// A second link with the same name gets a new id, the first keeps its own.
	result, _ = app.AddLink(
		LinkInput{URL: "https://other.test", Name: "My Site", Category: "docs"},
	)
	if result.ID != "my-site-2" {
		t.Errorf("second id = %q", result.ID)
	}
}

func TestUpdateAndDeleteLink(t *testing.T) {
	app, _, _ := newTestApp(t)
	if err := app.usage.RecordOpen("links:mdn"); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(app.iconsDir(), 0o700); err != nil {
		t.Fatal(err)
	}

	mdnIcon := filepath.Join(app.iconsDir(), "mdn.png")
	if err := os.WriteFile(mdnIcon, solidPNG(t, 8), 0o600); err != nil {
		t.Fatal(err)
	}

	result, err := app.UpdateLink(
		"mdn",
		LinkInput{URL: "https://github.com", Name: "MDN", Category: "docs"},
	)
	if err != nil || result.Duplicate == nil || result.Duplicate.ID != "github" {
		t.Errorf("update to a duplicate: %+v %v", result, err)
	}

	result, err = app.UpdateLink("mdn", LinkInput{
		URL:      "https://developer.mozilla.org/en-US/",
		Name:     "MDN Web Docs",
		Category: "dev",
	})
	if err != nil || result.ID != "mdn" || len(result.Problems) != 0 {
		t.Fatalf("update: %+v %v", result, err)
	}

	link, _ := app.findLink("mdn")
	if link.Name != "MDN Web Docs" || link.Category != "dev" {
		t.Errorf("stored = %+v", link)
	}

	_, err = app.UpdateLink("nope", LinkInput{})
	if !errors.Is(err, errUnknownItem) {
		t.Errorf("unknown: %v", err)
	}

	if err := app.DeleteLink("mdn"); err != nil {
		t.Fatal(err)
	}

	if _, ok := app.findLink("mdn"); ok {
		t.Error("still there")
	}

	if _, err := os.Stat(mdnIcon); !errors.Is(err, os.ErrNotExist) {
		t.Error("the icon file survived the delete")
	}

	if app.Usage().Opens["links:mdn"] != 0 {
		t.Error("the usage survived the delete")
	}

	if err := app.DeleteLink("mdn"); !errors.Is(err, errUnknownItem) {
		t.Errorf("second delete: %v", err)
	}
}

func TestAppCRUDAndHiding(t *testing.T) {
	app, _, _ := newTestApp(t)
	bundle := fakeBundle(t, app, "Alpha", "com.example.alpha")

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

	if err := app.HideApp("app-alpha"); err != nil {
		t.Fatal(err)
	}

	if err := app.HideApp("app-alpha"); err != nil {
		t.Errorf("hiding twice must be idempotent: %v", err)
	}

	hidden := false
	for _, item := range app.Items(tabApps) {
		if item.ID == "app-alpha" && item.Hidden {
			hidden = true
		}
	}

	if !hidden {
		t.Error("app-alpha is not hidden")
	}

	if err := app.UnhideApp("app-alpha"); err != nil {
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

func TestCategories(t *testing.T) {
	app, _, _ := newTestApp(t)

	view, err := app.AddCategory(tabLinks, " Mi ecosistema ")
	wrongView := err != nil || view.ID != "mi-ecosistema" ||
		view.Name != "Mi ecosistema" || view.Tab != tabLinks
	if wrongView {
		t.Fatalf("add: %+v %v", view, err)
	}

	view, err = app.AddCategory(tabApps, "Favoritos")
	if err != nil || view.ID != "favoritos-2" {
		t.Errorf("reserved name: %+v %v", view, err)
	}

	if _, err := app.AddCategory("other", "X"); err == nil {
		t.Error("a bad tab was accepted")
	}

	if _, err := app.AddCategory(tabLinks, ""); err == nil {
		t.Error("an empty name was accepted")
	}

	renameErr := app.RenameCategory(tabLinks, "mi-ecosistema", "Ecosistema")
	if renameErr != nil {
		t.Fatal(renameErr)
	}

	inUseErr := app.DeleteCategory(tabLinks, "dev")
	if !errors.Is(inUseErr, errCategoryInUse) {
		t.Errorf("delete in use: %v", inUseErr)
	}

	if err := app.DeleteCategory(tabLinks, "mi-ecosistema"); err != nil {
		t.Fatal(err)
	}

	secondErr := app.DeleteCategory(tabLinks, "mi-ecosistema")
	if !errors.Is(secondErr, errUnknownItem) {
		t.Errorf("second delete: %v", secondErr)
	}
}

func TestInspectURL(t *testing.T) {
	app, _, _ := newTestApp(t)
	allowPrivate(t, app)
	server := iconAndPage(t)

	if _, err := app.InspectURL("not a url"); err == nil {
		t.Error("garbage was accepted")
	}

	bareHost := strings.TrimPrefix(server.URL, "http://")

	draft, err := app.InspectURL(bareHost + "/page")
	if err != nil {
		t.Fatal(err)
	}

	if draft.URL != "https://"+bareHost+"/page" || draft.Insecure {
		t.Errorf("a bare host gets https: %+v", draft)
	}

	draft, err = app.InspectURL(server.URL + "/page")
	if err != nil {
		t.Fatal(err)
	}

	wrongDraft := draft.Name != "Example" ||
		draft.Description != "An example site" ||
		!draft.Insecure || draft.Host != "127.0.0.1"
	if wrongDraft {
		t.Errorf("draft = %+v", draft)
	}

	if !strings.HasPrefix(draft.IconDataURL, "data:image/png;base64,") {
		t.Errorf("icon = %.40q", draft.IconDataURL)
	}

	noDuplicates := draft.Duplicate != nil || draft.SameHost == nil ||
		len(draft.SameHost) != 0
	if noDuplicates {
		t.Errorf("duplicates = %+v %+v", draft.Duplicate, draft.SameHost)
	}

	mine := LinkInput{URL: server.URL + "/page", Name: "Mine", Category: "dev"}
	if _, err := app.AddLink(mine); err != nil {
		t.Fatal(err)
	}

	app.background.Wait()

	draft, _ = app.InspectURL(server.URL + "/page/")
	afterAdding := draft.Duplicate == nil || draft.Duplicate.ID != "mine" ||
		len(draft.SameHost) != 1
	if afterAdding {
		t.Errorf("after adding: %+v %+v", draft.Duplicate, draft.SameHost)
	}
}

// Review Focus 4: an icon lands in the background while the file is edited
// by hand; the hand edit survives, the icon is written, the page is told.
func TestBackgroundIconFetchNeverTouchesTheLibrary(t *testing.T) {
	app, win, _ := newTestApp(t)
	allowPrivate(t, app)
	server := iconAndPage(t)

	mine := LinkInput{URL: server.URL + "/page", Name: "Mine", Category: "dev"}
	if _, err := app.AddLink(mine); err != nil {
		t.Fatal(err)
	}

	// The hand edit: another link appended straight into the file.
	data, err := os.ReadFile(app.library.Path())
	if err != nil {
		t.Fatal(err)
	}

	handEntry := "\n[[links]]\nid = \"by-hand\"\nname = \"By hand\"\n" +
		"url = \"https://hand.test\"\ncategory = \"dev\"\n"
	edited := string(data) + handEntry
	writeErr := os.WriteFile(app.library.Path(), []byte(edited), 0o600)
	if writeErr != nil {
		t.Fatal(writeErr)
	}

	waitForCall(t, win, "emit:icons:mine")
	app.background.Wait()

	if _, err := os.Stat(filepath.Join(app.iconsDir(), "mine.png")); err != nil {
		t.Errorf("icon not written: %v", err)
	}

	if _, ok := app.findLink("by-hand"); !ok {
		t.Error("the hand edit was lost")
	}

	if items := app.Items(tabLinks); items[6].IconURL == "" {
		t.Errorf("IconURL not filled after the fetch: %+v", items[6])
	}

	// The listing above also starts a fetch for every other link still
	// without an icon (the seed's); let those settle before taking the
	// baseline below, or their own completions would look like a retry.
	app.background.Wait()

	// One attempt per id per run: listing again fetches nothing new.
	before := win.joined()
	app.Items(tabLinks)
	app.background.Wait()
	if win.joined() != before {
		t.Error("a second fetch was started for an icon that exists")
	}
}

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
