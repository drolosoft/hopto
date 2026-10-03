package app

import (
	"strings"
	"testing"

	"github.com/drolosoft/hopto/internal/platform"
)

// withHotkeys records the two registrations as the native side would,
// and forgets them after the test: the table is global.
func withHotkeys(t *testing.T, apps, links int32) {
	t.Helper()

	recordHotkeyStatus(platform.HotkeyApps, apps)
	recordHotkeyStatus(platform.HotkeyLinks, links)

	t.Cleanup(func() {
		hotkeyStatuses.Lock()
		clear(hotkeyStatuses.byID)
		hotkeyStatuses.Unlock()
	})
}

// Before the system answers, a shortcut is pending, which is no problem.
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
	for call := range strings.SplitSeq(win.joined(), " ") {
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

	if len(open.calls) != 1 || open.calls[0][0] != platform.KeyboardSettingsURL {
		t.Errorf("open = %v", open.calls)
	}
}
