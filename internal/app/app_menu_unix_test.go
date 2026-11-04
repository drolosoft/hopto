//go:build !windows

package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/drolosoft/hopto/internal/library"
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
		"Arrancar al iniciar sesión|Icono||Salir de hopto"
	if got := strings.Join(titles, "|"); got != want {
		t.Errorf("titles = %q", got)
	}

	if menu.isChecked(menuLogin) {
		t.Error("login ticked with no agent on disk")
	}

	labels := menuLabelsFor(platform.LanguageEnglish)
	if labels.Quit != "Quit hopto" || labels.Icons["key"] != "Key" {
		t.Errorf("english = %+v", labels)
	}
}

// The Icon submenu offers every drawing, in the library's order and
// with a name in both languages, with the key ticked on a fresh library;
// the item shows that same drawing, as a template.
func TestMenuIconSubmenu(t *testing.T) {
	app, _, _ := newTestApp(t)
	menu := withMenu(t, app)

	entries := menu.submenu("Icon")
	if len(entries) != len(library.MenubarIcons) {
		t.Fatalf("entries = %+v", entries)
	}

	for index, entry := range entries {
		id := library.MenubarIcons[index]
		spanish := menuLabelsFor(platform.LanguageSpanish).Icons[id]
		if entry.Label == "" || spanish == "" || entry.Tag != menuIconFirst+index {
			t.Errorf("%s: entry = %+v, spanish %q", id, entry, spanish)
		}

		if entry.Checked != (id == library.MenubarIconKey) {
			t.Errorf("%s: ticked = %v", id, entry.Checked)
		}

		// Each entry shows its drawing: the generic one the system
		// symbol (the empty icon), the colour one not as a template.
		if entry.Icon == nil {
			t.Errorf("%s: no drawing", id)
			continue
		}

		generic := id == library.MenubarIconGeneric
		colour := id == library.MenubarIconColour
		wantTemplate := !generic && !colour
		if entry.Icon.Empty() != generic || entry.Icon.Template != wantTemplate {
			t.Errorf("%s: icon empty=%v template=%v",
				id, entry.Icon.Empty(), entry.Icon.Template)
		}
	}

	icon := menu.lastIcon()
	if icon.Empty() || !icon.Template {
		t.Errorf("icon at start = %d/%d bytes, template %v",
			len(icon.Small), len(icon.Large), icon.Template)
	}
}

// A drawing picked from the submenu is saved in the file, ticked alone
// and shown: the frog in colour is not a template, the generic one is
// the empty icon that means the system symbol. A file that refuses the
// write leaves the ticks and the item as they were.
func TestMenuIconChoice(t *testing.T) {
	app, _, _ := newTestApp(t)
	menu := withMenu(t, app)

	tagOf := func(id string) int {
		for index, other := range library.MenubarIcons {
			if other == id {
				return menuIconFirst + index
			}
		}

		t.Fatalf("no tag for %s", id)

		return 0
	}

	app.menuAction(tagOf(library.MenubarIconColour))
	if got := app.library.Snapshot().Settings.MenubarIcon; got != "colour" {
		t.Fatalf("saved = %q", got)
	}

	if !menu.isChecked(tagOf("colour")) || menu.isChecked(tagOf("key")) {
		t.Error("the tick did not move to the colour one")
	}

	if icon := menu.lastIcon(); icon.Empty() || icon.Template {
		t.Errorf("colour icon = %d bytes, template %v",
			len(icon.Large), icon.Template)
	}

	app.menuAction(tagOf(library.MenubarIconGeneric))
	if icon := menu.lastIcon(); !icon.Empty() {
		t.Error("the generic choice did not ask for the system symbol")
	}

	// A broken file: the store refuses the write.
	path := filepath.Join(platform.DataDir(app.home), "library.toml")
	if err := os.WriteFile(path, []byte("version = \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	calls := menu.icons
	app.menuAction(tagOf(library.MenubarIconFace))
	if menu.icons != calls || menu.isChecked(tagOf("face")) {
		t.Error("a refused save changed the item")
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

// With the open panel up, Help from the menu must not reach the page
// either: the help would paint over a window the user cannot reach while
// the sheet is on it.
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
	icon    native.Icon
	icons   int
}

// Install keeps the entries and their ticks, submenus included, as the
// real item draws them.
func (m *fakeMenu) Install(items []native.MenuItem) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.items = items
	m.checked = map[int]bool{}
	m.remember(items)
}

// remember keeps the ticks of a list of entries and of their submenus.
func (m *fakeMenu) remember(items []native.MenuItem) {
	for _, item := range items {
		m.checked[item.Tag] = item.Checked
		m.remember(item.Children)
	}
}

// SetIcon keeps the last drawing and counts the calls.
func (m *fakeMenu) SetIcon(icon native.Icon) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.icon = icon
	m.icons++
}

// lastIcon reads the last drawing under the lock.
func (m *fakeMenu) lastIcon() native.Icon {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.icon
}

// submenu finds the entries of the submenu with a label, nil without it.
func (m *fakeMenu) submenu(label string) []native.MenuItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, item := range m.items {
		if item.Label == label {
			return item.Children
		}
	}

	return nil
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
