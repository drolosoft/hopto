package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// looseBundle writes a minimal .app somewhere outside the app roots.
func looseBundle(t *testing.T, name string) string {
	t.Helper()

	bundle := filepath.Join(t.TempDir(), "Downloads", name+".app")
	contents := filepath.Join(bundle, "Contents")
	if err := os.MkdirAll(contents, 0o755); err != nil {
		t.Fatal(err)
	}

	plist := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0">` +
		`<dict><key>CFBundleIdentifier</key>` +
		`<string>com.example.loose</string></dict></plist>`
	infoPath := filepath.Join(contents, "Info.plist")
	if err := os.WriteFile(infoPath, []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}

	return bundle
}

// Cancelling the panel answers an empty draft, and the window floats
// again afterwards, with the focus back.
func TestPickAppWithdrawsAlwaysOnTop(t *testing.T) {
	app, win, _ := newTestApp(t)
	app.visible = true

	draft, err := app.PickApp()
	if err != nil || draft.Path != "" {
		t.Fatalf("cancel: %+v %v", draft, err)
	}

	want := "ontop:false pick:/Applications ontop:true activate"
	if got := win.joined(); got != want {
		t.Errorf("calls = %q, want %q", got, want)
	}
}

// A picked bundle comes back with its name, bundle id and icon, and a
// discovered copy of the same app is named as the duplicate.
func TestPickAppReadsTheBundle(t *testing.T) {
	app, win, _ := newTestApp(t)
	app.visible = true
	win.pickPath = fakeBundle(t, app, "Alpha", "com.example.alpha") + "/"

	draft, err := app.PickApp()
	if err != nil {
		t.Fatal(err)
	}

	wrong := draft.Name != "Alpha" ||
		draft.BundleID != "com.example.alpha" ||
		strings.HasSuffix(draft.Path, "/") ||
		draft.Problem != "" ||
		!strings.HasPrefix(draft.IconDataURL, "data:image/png;base64,")
	if wrong {
		t.Errorf("draft = %+v", draft)
	}

	if draft.Duplicate == nil || draft.Duplicate.ID != "app-alpha" {
		t.Errorf("duplicate = %+v", draft.Duplicate)
	}
}

// A bundle outside /Applications, /System/Applications and
// ~/Applications is reported, not silently accepted.
func TestPickAppOutsideTheRoots(t *testing.T) {
	app, win, _ := newTestApp(t)
	app.visible = true
	win.pickPath = looseBundle(t, "Loose")

	draft, err := app.PickApp()
	if err != nil {
		t.Fatal(err)
	}

	if draft.Problem != "app.path" || draft.Name != "Loose" {
		t.Errorf("draft = %+v", draft)
	}

	if draft.Duplicate != nil {
		t.Errorf("duplicate = %+v", draft.Duplicate)
	}
}

// An error from the panel is passed on, and the window floats again.
func TestPickAppPassesErrorsOn(t *testing.T) {
	app, win, _ := newTestApp(t)
	app.visible = true
	win.pickErr = errors.New("panel broke")

	if _, err := app.PickApp(); err == nil {
		t.Fatal("expected the panel's error")
	}

	if !strings.HasSuffix(win.joined(), "ontop:true activate") {
		t.Errorf("calls = %q", win.joined())
	}
}

// With the sheet up, neither Hide (a blur from the page) nor the
// shortcut hides the window; once it closes, both work again.
func TestHideAndToggleWaitForTheDialog(t *testing.T) {
	app, win, _ := newTestApp(t)
	app.toggle(tabApps)

	if err := app.beginDialog(); err != nil {
		t.Fatalf("the first dialog was refused: %v", err)
	}

	app.Hide()
	app.toggle(tabApps)

	if strings.Contains(win.joined(), "hide") {
		t.Errorf("hidden under a dialog: %q", win.joined())
	}

	if _, err := app.PickApp(); !errors.Is(err, errDialogBusy) {
		t.Errorf("second dialog: %v", err)
	}

	app.endDialog()
	app.Hide()

	if !strings.HasSuffix(win.joined(), "hide") {
		t.Errorf("not hidden after the dialog: %q", win.joined())
	}
}

// A hotkey hide landing just before PickApp must not attach the sheet to
// a hidden window: it would never get an answer, leaving dialogOpen
// stuck refusing toggle, Hide and reopen until Quit.
func TestPickAppRefusesOnAHiddenPanel(t *testing.T) {
	app, win, _ := newTestApp(t)

	if _, err := app.PickApp(); !errors.Is(err, errPanelHidden) {
		t.Errorf("hidden panel: %v", err)
	}

	if app.dialogOpen {
		t.Error("dialogOpen left true")
	}

	if strings.Contains(win.joined(), "ontop") {
		t.Errorf("touched AlwaysOnTop: %q", win.joined())
	}
}
