package main

import "log"

// keyboardSettingsURL opens System Settings on the Keyboard pane, where
// Keyboard Shortcuts → Spotlight holds the Finder's ⌘⌥Space.
const keyboardSettingsURL = "x-apple.systempreferences:" +
	"com.apple.Keyboard-Settings.extension"

// The states of a shortcut as the welcome shows them.
const (
	hotkeyRegistered = "registered"
	hotkeyTaken      = "taken"
	hotkeyFailed     = "failed"
	hotkeyPending    = "pending"
)

// HotkeyView is one global shortcut and what Carbon said about it. The
// spec is the one in the settings; a spec that did not parse was
// replaced by its default at registration and says so in the log.
type HotkeyView struct {
	Tab    string `json:"tab"`
	Spec   string `json:"spec"`
	State  string `json:"state"`
	Status int32  `json:"status"`
}

// WelcomeView is what the welcome needs: whether to show at all, whether
// this is the first run, the two shortcuts, and whether the Finder keeps
// ⌘⌥Space. Hotkeys is never nil.
type WelcomeView struct {
	Show           bool         `json:"show"`
	FirstRun       bool         `json:"firstRun"`
	Hotkeys        []HotkeyView `json:"hotkeys"`
	FinderConflict bool         `json:"finderConflict"`
}

// Welcome says whether the welcome is due: on the first run, or on any
// run where a shortcut cannot work, until the user dismisses it.
func (a *App) Welcome() WelcomeView {
	settings := a.library.Snapshot().Settings

	view := WelcomeView{
		FirstRun: a.firstRun,
		Hotkeys: []HotkeyView{
			hotkeyView(tabApps, settings.HotkeyApps, hotkeyApps),
			hotkeyView(tabLinks, settings.HotkeyLinks, hotkeyLinks),
		},
		FinderConflict: usesFinderShortcut(settings) &&
			finderSearchShortcutEnabled(a.symbolicHotkeys),
	}

	problem := view.FinderConflict
	for _, hotkey := range view.Hotkeys {
		if hotkey.State == hotkeyTaken || hotkey.State == hotkeyFailed {
			problem = true
		}
	}

	a.mu.Lock()
	dismissed := a.welcomeDismissed
	a.mu.Unlock()

	view.Show = !dismissed && (view.FirstRun || problem)

	return view
}

// hotkeyView turns Carbon's answer for one shortcut into its state.
func hotkeyView(tab, spec string, id uint32) HotkeyView {
	view := HotkeyView{Tab: tab, Spec: spec, State: hotkeyPending}

	status, known := hotkeyStatus(id)
	if !known {
		return view
	}

	view.Status = status

	switch status {
	case 0:
		view.State = hotkeyRegistered
	case hotkeyExistsStatus:
		view.State = hotkeyTaken
	default:
		view.State = hotkeyFailed
	}

	return view
}

// PresentWelcome is what the page calls once it has loaded. Only on the
// first run does it show the panel by itself, whose `shown` then finds
// the welcome; a later run's problem (a taken shortcut, or the Finder's
// own ⌥⌘Space) is shown once the user opens the panel themselves,
// through checkWelcome on that `shown`. Popping the panel up unasked on
// every later launch is exactly what "Show Finder search window" already
// does with the default links shortcut on a stock Mac, which is the
// spec's ruling over the plan's own wider wording. Asking from the page,
// instead of from startup, means the event never arrives before anyone
// listens.
func (a *App) PresentWelcome() {
	view := a.Welcome()
	if !view.Show || !view.FirstRun {
		return
	}

	a.mu.Lock()
	visible := a.visible
	a.mu.Unlock()

	if visible {
		return
	}

	log.Printf("welcome: showing the panel")
	a.toggle(tabApps)
}

// DismissWelcome hides the welcome for the rest of this run.
func (a *App) DismissWelcome() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.welcomeDismissed = true
}

// OpenKeyboardSettings opens the pane where the Finder's shortcut can be
// turned off.
func (a *App) OpenKeyboardSettings() error {
	return a.open(keyboardSettingsURL)
}
