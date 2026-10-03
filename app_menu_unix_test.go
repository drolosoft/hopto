//go:build !windows

package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// These tests run hopto as hopto.app and toggle its LaunchAgent, a
// macOS fixture, so they run everywhere but Windows: there the login
// item is the user's real Run key, which a test must not touch.

// withMenu installs a fake menu bar item and points the login switch
// at a bundle inside the test's home.
func withMenu(t *testing.T, app *App) *fakeMenu {
	t.Helper()

	menu := &fakeMenu{}
	app.menu = menu
	bundle := filepath.Join(app.home, "Applications", "hopto.app")
	app.login.executable = func() (string, error) {
		return filepath.Join(bundle, "Contents", "MacOS", "hopto"), nil
	}

	app.installMenu()

	return menu
}

// The five entries in order, a rule before Quit, in the language of the
// settings, with the login tick read from the disk.
func TestMenuItems(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.language = languageSpanish
	menu := withMenu(t, app)

	titles := []string{}
	for _, item := range menu.items {
		titles = append(titles, item.Title)
	}

	want := "Abrir hopto|Ayuda|Editar library.toml|" +
		"Arrancar al iniciar sesión||Salir de hopto"
	if got := strings.Join(titles, "|"); got != want {
		t.Errorf("titles = %q", got)
	}

	if menu.isChecked(menuLogin) {
		t.Error("login ticked with no agent on disk")
	}

	if labels := menuLabelsFor(languageEnglish); labels.Quit != "Quit hopto" {
		t.Errorf("english = %+v", labels)
	}
}

// Review Focus 5: the entry writes and removes the agent and the tick
// follows the disk; outside a bundle it refuses and stays unticked.
func TestMenuLoginItemToggles(t *testing.T) {
	app, _, _ := newTestApp(t)
	menu := withMenu(t, app)

	app.menuAction(menuLogin)
	if !app.login.Enabled() || !menu.isChecked(menuLogin) {
		t.Fatal("first click did not enable")
	}

	app.menuAction(menuLogin)
	if app.login.Enabled() || menu.isChecked(menuLogin) {
		t.Fatal("second click did not disable")
	}

	app.login.executable = func() (string, error) {
		return filepath.Join(t.TempDir(), "hopto.test"), nil
	}

	app.menuAction(menuLogin)
	if app.login.Enabled() || menu.isChecked(menuLogin) {
		t.Error("enabled outside a bundle")
	}
}

// Open shows the panel once and never hides it; Help shows it and asks
// the page for the help; Edit opens the file; Quit quits.
func TestMenuOpenHelpEditQuit(t *testing.T) {
	app, win, open := newTestApp(t)
	withMenu(t, app)

	app.menuAction(menuOpen)
	app.menuAction(menuOpen)

	want := "center show activate emit:shown:apps activate"
	if got := win.joined(); got != want {
		t.Errorf("open twice = %q", got)
	}

	app.Hide()
	app.menuAction(menuHelp)
	if !strings.HasSuffix(win.joined(), "emit:shown:apps emit:help:true") {
		t.Errorf("help = %q", win.joined())
	}

	app.menuAction(menuEdit)
	if len(open.calls) != 1 || open.calls[0][0] != "-t" {
		t.Errorf("edit = %v", open.calls)
	}

	app.menuAction(menuQuit)
	if !strings.HasSuffix(win.joined(), "quit") {
		t.Errorf("quit = %q", win.joined())
	}
}

// Plan 3 minor (b): with the open panel up, Help from the menu must not
// reach the page either: the help would paint over a window the user
// cannot reach while the sheet is on it.
func TestMenuHelpWaitsForTheDialog(t *testing.T) {
	app, win, _ := newTestApp(t)
	withMenu(t, app)
	app.visible = true

	if err := app.beginDialog(); err != nil {
		t.Fatalf("dialog refused: %v", err)
	}
	defer app.endDialog()

	app.menuAction(menuHelp)
	if strings.Contains(win.joined(), "emit:help") {
		t.Errorf("help under a dialog: %q", win.joined())
	}
}
