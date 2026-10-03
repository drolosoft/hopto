package main

import (
	"errors"
	"strings"
	"testing"
)

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
