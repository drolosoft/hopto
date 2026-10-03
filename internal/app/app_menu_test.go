package app

import (
	"strings"
	"testing"
)

// With the open panel up the menu does not touch the window either.
func TestShowPanelWaitsForTheDialog(t *testing.T) {
	app, win, _ := newTestApp(t)
	app.visible = true

	if err := app.beginDialog(); err != nil {
		t.Fatalf("dialog refused: %v", err)
	}
	defer app.endDialog()

	app.showPanel(tabApps)
	if strings.Contains(win.joined(), "show") {
		t.Errorf("shown under a dialog: %q", win.joined())
	}
}
