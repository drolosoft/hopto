package main

import (
	"log"

	"github.com/wailsapp/wails/v2/pkg/options"
)

// showFromOutside is what a launch of the running hopto ends in: a
// reopen (Alfred, `open -a`, a double click in the Finder or the Dock)
// or a second process that Wails' single instance lock sends back here.
// It shows the panel like the apps shortcut but never hides it: with the
// panel up it only brings it forward, on the tab it is on, as launching
// an app that is already open does. With the file dialog up it does
// nothing, like toggle.
func (a *App) showFromOutside() {
	a.mu.Lock()
	defer a.mu.Unlock()

	log.Printf("show from outside: visible=%v", a.visible)

	// A second launch can arrive before startup has handed over the
	// window.
	if a.window == nil || a.dialogOpen {
		return
	}

	if a.visible {
		a.window.Activate()
		return
	}

	a.placeWindow()
	a.window.Show()
	a.window.Activate()
	a.visible = true
	a.tab = tabApps

	// The same reset the shortcuts trigger.
	a.window.Emit("shown", tabApps)
}

// secondInstance is Wails' OnSecondInstanceLaunch: `open -n`, or a
// launcher set up to start a new copy, ran hopto again, and that copy
// has already quit. Wails calls it on a goroutine of its own. The
// arguments are counted, not logged: they could be anything.
func (a *App) secondInstance(data options.SecondInstanceData) {
	log.Printf("second instance launched (%d arguments)", len(data.Args))
	a.showFromOutside()
}
