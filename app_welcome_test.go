package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withHotkeys records the two registrations as Carbon would, and forgets
// them after the test: the table is global, like the C callback.
func withHotkeys(t *testing.T, apps, links int32) {
	t.Helper()

	recordHotkeyStatus(hotkeyApps, apps)
	recordHotkeyStatus(hotkeyLinks, links)

	t.Cleanup(func() {
		hotkeyStatuses.Lock()
		clear(hotkeyStatuses.byID)
		hotkeyStatuses.Unlock()
	})
}

// finderOff writes a symbolic hotkeys file with 65 turned off.
func finderOff(t *testing.T, app *App) {
	t.Helper()

	off := `<key>65</key><dict><key>enabled</key><false/></dict>`
	if err := os.MkdirAll(filepath.Dir(app.symbolicHotkeys), 0o700); err != nil {
		t.Fatal(err)
	}

	err := os.WriteFile(app.symbolicHotkeys, symbolicPrefs(off), 0o600)
	if err != nil {
		t.Fatal(err)
	}
}

// The run that creates library.toml is the first; the welcome shows
// until dismissed and never in the next run when all is well.
func TestWelcomeOnTheFirstRunOnly(t *testing.T) {
	app, _, _ := newTestApp(t)
	finderOff(t, app)
	withHotkeys(t, 0, 0)

	view := app.Welcome()
	wrong := !view.Show || !view.FirstRun || view.FinderConflict ||
		len(view.Hotkeys) != 2 ||
		view.Hotkeys[0].Tab != tabApps ||
		view.Hotkeys[0].Spec != "cmd+shift+space" ||
		view.Hotkeys[1].State != "registered"
	if wrong {
		t.Fatalf("first run = %+v", view)
	}

	app.DismissWelcome()
	if app.Welcome().Show {
		t.Error("shown after being dismissed")
	}

	again := newApp(
		app.home, app.dataDir, &fakeWindow{}, app.open, languageEnglish,
	)
	if view := again.Welcome(); view.Show || view.FirstRun {
		t.Errorf("second run = %+v", view)
	}
}

// A taken shortcut or the Finder's window bring the welcome back on a
// later run, once.
func TestWelcomeReportsHotkeyProblems(t *testing.T) {
	first, _, _ := newTestApp(t)
	app := newApp(
		first.home, first.dataDir, &fakeWindow{}, first.open, languageEnglish,
	)
	withHotkeys(t, 0, hotkeyExistsStatus)

	view := app.Welcome()
	if !view.Show || view.FirstRun || view.Hotkeys[1].State != "taken" {
		t.Errorf("taken = %+v", view)
	}

	if !view.FinderConflict {
		t.Error("no symbolic hotkeys file means 65 is on, and links is ⌘⌥Space")
	}

	recordHotkeyStatus(hotkeyLinks, -50)
	state := app.Welcome().Hotkeys[1]
	if state.State != "failed" || state.Status != -50 {
		t.Errorf("failed = %+v", state)
	}

	app.DismissWelcome()
	if app.Welcome().Show {
		t.Error("shown twice in one run")
	}
}

// Before Carbon answers, a shortcut is pending, which is no problem.
func TestWelcomeWhileRegistering(t *testing.T) {
	app, _, _ := newTestApp(t)
	finderOff(t, app)
	app.firstRun = false

	view := app.Welcome()
	if view.Show || view.Hotkeys[0].State != "pending" {
		t.Errorf("pending = %+v", view)
	}
}

// PresentWelcome opens the panel when the welcome is due and the panel
// is hidden; never when it is up, never when nothing is due.
func TestPresentWelcome(t *testing.T) {
	app, win, _ := newTestApp(t)
	finderOff(t, app)
	withHotkeys(t, 0, 0)

	app.PresentWelcome()
	app.PresentWelcome()

	want := "center show activate emit:shown:apps"
	if got := win.joined(); got != want {
		t.Errorf("calls = %q", got)
	}

	app.Hide()
	app.DismissWelcome()
	app.PresentWelcome()

	// strings.Count would also match the "show" inside the "shown" event
	// that the first appearance already emitted, so the calls are split
	// into tokens first: what matters is how many times Show() itself
	// ran, not how many times the substring appears anywhere in the log.
	shown := 0
	for _, call := range strings.Split(win.joined(), " ") {
		if call == "show" {
			shown++
		}
	}

	if shown != 1 {
		t.Errorf("shown again: %q", win.joined())
	}
}

// The settings button opens the Keyboard pane through `open`.
func TestOpenKeyboardSettings(t *testing.T) {
	app, _, open := newTestApp(t)

	if err := app.OpenKeyboardSettings(); err != nil {
		t.Fatal(err)
	}

	if len(open.calls) != 1 || open.calls[0][0] != keyboardSettingsURL {
		t.Errorf("open = %v", open.calls)
	}
}
