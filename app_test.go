package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/drolosoft/hopto/internal/platform"
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

// testHome is a temporary home folder for one test. It is removed with a
// few retries rather than through t.TempDir: on Windows a folder the
// test just created can be held open for a moment by the indexer or an
// antivirus, and the first RemoveAll then fails with "being used by
// another process". The retries wait for the handle to go.
func testHome(t *testing.T) string {
	t.Helper()

	home, err := os.MkdirTemp("", "hopto-test-home-")
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		removeWithRetries(t, home)
	})

	return home
}

// removeWithRetries removes a folder, trying again a few times when
// Windows reports it as in use; the last error fails the test.
func removeWithRetries(t *testing.T, path string) {
	t.Helper()

	var err error
	for attempt := 0; attempt < 10; attempt++ {
		err = os.RemoveAll(path)
		if err == nil {
			return
		}

		time.Sleep(100 * time.Millisecond)
	}

	t.Errorf("remove %s: %v", path, err)
}

// closeIcons releases the icons folder an App holds open once the test
// ends. Windows cannot remove a folder while a handle on it is open, and
// testHome removes the temp home. Cleanups run last in, first out, so this
// one runs before that removal. The background work goes first because it
// may still be reading an icon.
func closeIcons(t *testing.T, app *App) {
	t.Helper()

	t.Cleanup(func() {
		app.background.Wait()

		if app.iconHandler != nil {
			_ = app.iconHandler.Close()
		}
	})
}

// newTestApp builds an App over a temp home, with discovery pointed at
// folders inside it and English texts.
func newTestApp(t *testing.T) (*App, *fakeWindow, *fakeOpen) {
	t.Helper()

	home := testHome(t)
	win := &fakeWindow{}
	open := &fakeOpen{}

	app := newApp(
		home, platform.DataDir(home), win, open.run, platform.LanguageEnglish,
	)
	app.appRoots = []string{filepath.Join(home, "Applications")}
	app.edgeDir = filepath.Join(home, "Applications", "Edge Apps.localized")
	app.offline = true

	// The real screen readers wait on the main thread, which go test
	// never runs: no screen under the window, and one display attached.
	app.windowDisplay = func() uint32 { return 0 }
	app.attachedDisplays = func() []uint32 { return []uint32{1} }

	closeIcons(t, app)

	return app, win, open
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
	app.language = platform.LanguageSpanish

	settings := app.Settings()
	wrongSettings := settings.Language != platform.LanguageSpanish ||
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
