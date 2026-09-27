package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
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

// slowIconPage is iconAndPage with the icon's size and reply delay under
// the test's control, so a fetch against it can be kept in flight for as
// long as the test needs, and told apart from another server's icon by
// its pixel size.
func slowIconPage(
	t *testing.T, side int, delay time.Duration,
) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc(
		"GET /icon.png",
		func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(delay)
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write(solidPNG(t, side))
		},
	)
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head><title>Race</title>
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

	// A second, unrelated bundle: hiding must be exercised on an app that
	// stays discovered, not on "app-alpha", which TestAHandAddedAppHides
	// ItsDiscoveredTwin covers as the hand-added "alpha"'s twin (Task 10
	// leaves a twin out of Items entirely, so it cannot be hidden there).
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

// While library.toml is broken, the page still shows the last good
// snapshot, but its ids may not be the user's once the file is fixed; a
// background fetch started for them now would leave the seed's icons
// behind in the real icons/ folder for good.
func TestNoIconFetchWhileReadOnly(t *testing.T) {
	app, win, _ := newTestApp(t)
	allowPrivate(t, app)

	broken := []byte("version = 1\n\n[[links]\nbroken\n")
	if err := os.WriteFile(app.library.Path(), broken, 0o600); err != nil {
		t.Fatal(err)
	}

	items := app.Items(tabLinks)
	if len(items) == 0 {
		t.Fatal("the last good snapshot did not stay on screen")
	}

	app.background.Wait()

	if strings.Contains(win.joined(), "emit:icons") {
		t.Error("an icon fetch ran while the library is read only")
	}

	entries, err := os.ReadDir(app.iconsDir())
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}

	if len(entries) != 0 {
		t.Errorf("icons written to disk while read only: %v", entries)
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

// Review finding 1: a fetch started by an edit that is itself overtaken
// by a second edit must not win the race to disk. AddLink starts a fetch
// against a slow server; UpdateLink immediately points the same link at
// a fast one. Whichever answers last, the fast server's icon (a different
// size, so the test can tell) must be the one that survives on disk, and
// the page must hear about it exactly once.
func TestUpdateLinkDropsAStaleInFlightFetch(t *testing.T) {
	app, win, _ := newTestApp(t)
	allowPrivate(t, app)

	const slowSide = 64
	const fastSide = 32

	slow := slowIconPage(t, slowSide, 300*time.Millisecond)
	fast := slowIconPage(t, fastSide, 0)

	added := LinkInput{URL: slow.URL + "/page", Name: "Racer", Category: "dev"}
	result, err := app.AddLink(added)
	if err != nil {
		t.Fatal(err)
	}

	edited := LinkInput{URL: fast.URL + "/page", Name: "Racer", Category: "dev"}
	if _, err := app.UpdateLink(result.ID, edited); err != nil {
		t.Fatal(err)
	}

	app.background.Wait()

	iconPath := filepath.Join(app.iconsDir(), result.ID+".png")
	data, err := os.ReadFile(iconPath)
	if err != nil {
		t.Fatal(err)
	}

	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}

	if config.Width != fastSide {
		t.Errorf(
			"icon on disk is %d px, want the fast server's %d",
			config.Width, fastSide,
		)
	}

	wanted := "emit:icons:" + result.ID
	if emits := strings.Count(win.joined(), wanted); emits != 1 {
		t.Errorf("%q seen %d times among %q", wanted, emits, win.joined())
	}
}

// Review finding 2: two adds racing on the same name must not compute the
// same id. Ten concurrent AddLink calls with the same name must all
// succeed with ten distinct ids and ten links on disk.
func TestConcurrentAddLinkGetsDistinctIDs(t *testing.T) {
	app, _, _ := newTestApp(t)

	const concurrency = 10

	// The seed already has links of its own; count only what this test adds.
	before := len(app.library.Snapshot().Links)

	var wg sync.WaitGroup
	ids := make([]string, concurrency)
	errs := make([]error, concurrency)

	for i := range concurrency {
		wg.Add(1)

		go func() {
			defer wg.Done()

			url := "https://race-" + strconv.Itoa(i) + ".test"
			result, err := app.AddLink(
				LinkInput{URL: url, Name: "Race", Category: "dev"},
			)
			ids[i] = result.ID
			errs[i] = err
		}()
	}

	wg.Wait()

	seen := map[string]bool{}
	for i, err := range errs {
		if err != nil {
			t.Fatalf("add %d: %v", i, err)
		}

		if ids[i] == "" {
			t.Errorf("add %d: empty id", i)
		}

		if seen[ids[i]] {
			t.Errorf("id %q used twice", ids[i])
		}
		seen[ids[i]] = true
	}

	if len(seen) != concurrency {
		t.Errorf("got %d distinct ids, want %d", len(seen), concurrency)
	}

	added := len(app.library.Snapshot().Links) - before
	if added != concurrency {
		t.Errorf("stored %d new links, want %d", added, concurrency)
	}
}

// An update and an add racing to the same URL must never leave two links
// that open the same page: both checks have to run against the file as it
// is under the store's lock, not against a snapshot taken before it.
func TestUpdateLinkRacesAnAddOnTheSameURL(t *testing.T) {
	app, _, _ := newTestApp(t)

	for round := range 20 {
		target := fmt.Sprintf("https://race-%d.example.org", round)
		start := make(chan struct{})

		var racers sync.WaitGroup
		racers.Add(2)

		go func() {
			defer racers.Done()
			<-start

			_, _ = app.AddLink(LinkInput{
				URL:      target,
				Name:     fmt.Sprintf("Race %d", round),
				Category: "docs",
			})
		}()

		go func() {
			defer racers.Done()
			<-start

			_, _ = app.UpdateLink(
				"mdn",
				LinkInput{URL: target, Name: "MDN", Category: "docs"},
			)
		}()

		close(start)
		racers.Wait()

		same := 0
		for _, link := range app.library.Snapshot().Links {
			if library.NormalizeURL(link.URL) == library.NormalizeURL(target) {
				same++
			}
		}

		if same != 1 {
			t.Fatalf("round %d: %d links point at %s", round, same, target)
		}

		// mdn moves to a URL of its own before the next round, so the next
		// race starts from the same shape.
		_, err := app.UpdateLink("mdn", LinkInput{
			URL:      fmt.Sprintf("https://mdn-%d.example.org", round),
			Name:     "MDN",
			Category: "docs",
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// The editor has no icon field, so an update without a hint keeps the
// one the link had; an explicit hint still replaces it.
func TestUpdateLinkKeepsTheIconHint(t *testing.T) {
	app, _, _ := newTestApp(t)

	result, err := app.UpdateLink("github", LinkInput{
		URL:      "https://github.com",
		Name:     "GitHub",
		Category: "dev",
		Keywords: []string{"git"},
	})
	if err != nil || result.ID != "github" {
		t.Fatalf("update: %+v %v", result, err)
	}

	link, _ := app.findLink("github")
	if link.Icon != "sh:github-light" || len(link.Keywords) != 1 {
		t.Errorf("stored = %+v", link)
	}

	_, err = app.UpdateLink("github", LinkInput{
		URL:      "https://github.com",
		Name:     "GitHub",
		Category: "dev",
		Icon:     "sh:github",
	})
	if err != nil {
		t.Fatal(err)
	}

	if link, _ := app.findLink("github"); link.Icon != "sh:github" {
		t.Errorf("explicit hint not stored: %q", link.Icon)
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
