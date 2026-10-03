package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/drolosoft/hopto/internal/library"
)

// window.json: a missing file and a broken one both read as "no display",
// and what is written reads back. Its mode is checked in
// screen_unix_test.go.
func TestWindowState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hopto", windowFile)

	if got := readWindowState(path); got != 0 {
		t.Errorf("missing file = %d, want 0", got)
	}

	if err := writeWindowState(path, 69733378); err != nil {
		t.Fatal(err)
	}

	if got := readWindowState(path); got != 69733378 {
		t.Errorf("round trip = %d, want 69733378", got)
	}

	broken := []string{"{", `{"display":"two"}`, `{"display":-1}`, "[]"}
	for _, content := range broken {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}

		if got := readWindowState(path); got != 0 {
			t.Errorf("%s = %d, want 0", content, got)
		}
	}
}

// The rule of the setting: "last" is the remembered display while it is
// attached and the main screen otherwise; "mouse" and "main" ignore what
// was remembered.
func TestScreenChoice(t *testing.T) {
	attached := []uint32{1, 7}

	cases := []struct {
		name        string
		setting     string
		remembered  uint32
		wantMode    string
		wantDisplay uint32
	}{
		{"last, still attached", screenLast, 7, screenLast, 7},
		{"last, unplugged", screenLast, 9, screenMain, 0},
		{"last, never shown", screenLast, 0, screenMain, 0},
		{"mouse ignores the memory", screenMouse, 7, screenMouse, 0},
		{"main ignores the memory", screenMain, 7, screenMain, 0},
	}

	for _, tc := range cases {
		mode, display := screenChoice(tc.setting, tc.remembered, attached)
		if mode != tc.wantMode || display != tc.wantDisplay {
			t.Errorf("%s: %s %d, want %s %d",
				tc.name, mode, display, tc.wantMode, tc.wantDisplay)
		}
	}
}

// The display the panel hid on, by Esc (Hide) or by its own shortcut
// (toggle), is where the next show centres it, also in a new run; a hide
// while the window is on no screen keeps the last one.
func TestPanelReturnsToItsDisplay(t *testing.T) {
	app, win, _ := newTestApp(t)
	app.attachedDisplays = func() []uint32 { return []uint32{1, 7, 9} }
	path := filepath.Join(app.dataDir, windowFile)

	app.windowDisplay = func() uint32 { return 7 }
	app.toggle(tabApps)
	app.Hide()

	if got := readWindowState(path); got != 7 {
		t.Fatalf("after Hide window.json = %d, want 7", got)
	}

	app.toggle(tabApps)
	if win.centerMode != screenLast || win.centerDisplay != 7 {
		t.Errorf("centred by %s on %d", win.centerMode, win.centerDisplay)
	}

	app.windowDisplay = func() uint32 { return 9 }
	app.toggle(tabApps)

	if got := readWindowState(path); got != 9 {
		t.Errorf("after the shortcut window.json = %d, want 9", got)
	}

	app.windowDisplay = func() uint32 { return 0 }
	app.toggle(tabApps)
	app.Hide()

	if got := readWindowState(path); got != 9 {
		t.Errorf("a hide on no screen wrote %d", got)
	}

	again := newApp(
		app.home, app.dataDir, &fakeWindow{}, nil, languageEnglish,
	)
	if again.display != 9 {
		t.Errorf("a new run remembers %d, want 9", again.display)
	}
}

// An unplugged display sends the panel to the main screen, and the
// setting "mouse" brings back the rule hopto had before.
func TestPanelScreenSetting(t *testing.T) {
	app, win, _ := newTestApp(t)
	app.display = 9

	app.toggle(tabApps)
	if win.centerMode != screenMain || win.centerDisplay != 0 {
		t.Errorf("centred by %s on %d", win.centerMode, win.centerDisplay)
	}

	app.toggle(tabApps)

	err := app.library.Apply(func(lib *library.Library) error {
		lib.Settings.Screen = screenMouse
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	app.toggle(tabApps)
	if win.centerMode != screenMouse {
		t.Errorf("centred by %s, want mouse", win.centerMode)
	}
}
