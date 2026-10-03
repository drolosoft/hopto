//go:build !windows

package app

import (
	"testing"

	"github.com/drolosoft/hopto/internal/platform"
)

// These tests expect the Command shortcuts of the macOS seed and the
// Finder's own Option-Command-Space, so they run everywhere but Windows.

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
		app.home, app.dataDir, &fakeWindow{}, app.open, platform.LanguageEnglish,
	)
	closeIcons(t, again)

	if view := again.Welcome(); view.Show || view.FirstRun {
		t.Errorf("second run = %+v", view)
	}
}

// A taken shortcut or the Finder's window bring the welcome back on a
// later run, once.
func TestWelcomeReportsHotkeyProblems(t *testing.T) {
	first, _, _ := newTestApp(t)
	app := newApp(
		first.home, first.dataDir, &fakeWindow{}, first.open,
		platform.LanguageEnglish,
	)
	closeIcons(t, app)

	withHotkeys(t, 0, platform.HotkeyExistsStatus)

	view := app.Welcome()
	if !view.Show || view.FirstRun || view.Hotkeys[1].State != "taken" {
		t.Errorf("taken = %+v", view)
	}

	if !view.FinderConflict {
		t.Error("no symbolic hotkeys file means 65 is on, and links is ⌘⌥Space")
	}

	recordHotkeyStatus(platform.HotkeyLinks, -50)
	state := app.Welcome().Hotkeys[1]
	if state.State != "failed" || state.Status != -50 {
		t.Errorf("failed = %+v", state)
	}

	app.DismissWelcome()
	if app.Welcome().Show {
		t.Error("shown twice in one run")
	}
}

// On a later run, a taken shortcut or the Finder's own window must not
// open the panel by itself: with the stock ⌥⌘Space still bound to "Show
// Finder search window" and hopto's own default links shortcut of
// cmd+option+space, that would pop the panel up on every launch for any
// user who never touched either setting. The spec's ruling is that the
// welcome shows once; a later problem waits for the user's own `shown`.
func TestPresentWelcomeStaysQuietAfterTheFirstRun(t *testing.T) {
	first, _, _ := newTestApp(t)
	win := &fakeWindow{}
	app := newApp(
		first.home, first.dataDir, win, first.open, platform.LanguageEnglish,
	)
	closeIcons(t, app)

	withHotkeys(t, 0, platform.HotkeyExistsStatus)

	view := app.Welcome()
	if !view.Show || view.FirstRun {
		t.Fatalf("setup: expected a due problem on a later run, got %+v", view)
	}

	app.PresentWelcome()

	if got := win.joined(); got != "" {
		t.Errorf("opened the panel unasked: %q", got)
	}
}
