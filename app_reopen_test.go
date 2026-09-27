package main

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
)

// Opening hopto from outside while it runs: hidden, the panel comes up
// on the apps tab like the shortcut; shown, it only comes forward and
// keeps its tab; under the file dialog, nothing moves.
func TestShowFromOutside(t *testing.T) {
	cases := []struct {
		name       string
		visible    bool
		tab        string
		dialogOpen bool
		wantTab    string
		wantCalls  string
	}{
		{
			name: "hidden", tab: tabLinks, wantTab: tabApps,
			wantCalls: "center show activate emit:shown:apps",
		},
		{
			name: "visible", visible: true, tab: tabLinks,
			wantTab: tabLinks, wantCalls: "activate",
		},
		{
			name: "dialog open", visible: true, tab: tabLinks,
			dialogOpen: true, wantTab: tabLinks, wantCalls: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app, win, _ := newTestApp(t)
			app.visible = tc.visible
			app.tab = tc.tab
			app.dialogOpen = tc.dialogOpen

			app.showFromOutside()

			if !app.visible || app.tab != tc.wantTab {
				t.Errorf("visible %v tab %s", app.visible, app.tab)
			}

			if got := win.joined(); got != tc.wantCalls {
				t.Errorf("calls %q, want %q", got, tc.wantCalls)
			}
		})
	}
}

// A second copy launched before startup has handed over the window
// finds nothing to show and must not bring the first one down.
func TestSecondInstanceBeforeStartup(t *testing.T) {
	app, _, _ := newTestApp(t)
	app.window = nil

	app.secondInstance(options.SecondInstanceData{Args: []string{}})

	if app.visible {
		t.Error("shown without a window")
	}
}
