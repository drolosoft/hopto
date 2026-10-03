//go:build !windows

package main

import (
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/drolosoft/hopto/internal/native"
	"github.com/drolosoft/hopto/internal/platform"
)

// These tests read the menu's titles and its macOS entries, so they run
// everywhere but Windows. The login switch is a fake: the real
// LaunchAgent is tested inside the platform package.

// withMenu installs a fake menu bar item and a fake login agent.
func withMenu(t *testing.T, app *App) *fakeMenu {
	t.Helper()

	menu := &fakeMenu{}
	app.menu = menu
	app.login = &fakeLogin{}

	app.installMenu()

	return menu
}

// The five entries in order, a rule before Quit, in the language of the
// settings, with the login tick read from the disk.
func TestMenuItems(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.language = platform.LanguageSpanish
	menu := withMenu(t, app)

	titles := []string{}
	for _, item := range menu.items {
		titles = append(titles, item.Label)
	}

	want := "Abrir hopto|Ayuda|Editar library.toml|" +
		"Arrancar al iniciar sesión||Salir de hopto"
	if got := strings.Join(titles, "|"); got != want {
		t.Errorf("titles = %q", got)
	}

	if menu.isChecked(menuLogin) {
		t.Error("login ticked with no agent on disk")
	}

	labels := menuLabelsFor(platform.LanguageEnglish)
	if labels.Quit != "Quit hopto" {
		t.Errorf("english = %+v", labels)
	}
}

// The menu entry reaches the agent through menuAction and the tick
// follows what the agent says afterwards: a switch that works flips it,
// a refusal leaves the agent and the tick off.
func TestMenuLoginItemUsesTheAgent(t *testing.T) {
	cases := []struct {
		name        string
		err         error
		wantEnabled bool
	}{
		{"the agent accepts", nil, true},
		{"the agent refuses", errors.New("not in a bundle"), false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, _, _ := newTestApp(t)
			menu := withMenu(t, app)
			agent := &fakeLogin{err: tc.err}
			app.login = agent

			app.menuAction(menuLogin)
			if agent.Enabled() != tc.wantEnabled ||
				menu.isChecked(menuLogin) != tc.wantEnabled {
				t.Fatalf("first click: enabled=%v ticked=%v",
					agent.Enabled(), menu.isChecked(menuLogin))
			}

			if tc.err != nil {
				return
			}

			app.menuAction(menuLogin)
			if agent.Enabled() || menu.isChecked(menuLogin) {
				t.Fatal("second click did not disable")
			}
		})
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

// fakeLogin is a LoginAgent that only remembers what was asked of it. A
// canned err makes it refuse, as the real one does outside a bundle:
// the switch is left as it was.
type fakeLogin struct {
	enabled bool
	err     error
}

// Enabled reports the last switch.
func (f *fakeLogin) Enabled() bool { return f.enabled }

// Enable switches on unless the canned error refuses.
func (f *fakeLogin) Enable() error {
	if f.err != nil {
		return f.err
	}

	f.enabled = true

	return f.err
}

// Disable switches off unless the canned error refuses.
func (f *fakeLogin) Disable() error {
	if f.err != nil {
		return f.err
	}

	f.enabled = false

	return f.err
}

// fakeMenu records what App puts in the menu bar item.
type fakeMenu struct {
	mu      sync.Mutex
	items   []native.MenuItem
	checked map[int]bool
}

// Install keeps the entries and their ticks, as the real item draws them.
func (m *fakeMenu) Install(items []native.MenuItem) {
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
