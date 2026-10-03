package app

import (
	"log"

	"github.com/drolosoft/hopto/internal/platform"
)

// The states of a shortcut as the welcome shows them.
const (
	hotkeyRegistered = "registered"
	hotkeyTaken      = "taken"
	hotkeyFailed     = "failed"
	hotkeyPending    = "pending"
)

// HotkeyView is one global shortcut and what the system said when hopto
// registered it. Spec is the shortcut as the settings write it; one that
// did not parse was replaced by its default at registration and says so
// in the log.
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
			hotkeyView(tabApps, settings.HotkeyApps, platform.HotkeyApps),
			hotkeyView(tabLinks, settings.HotkeyLinks, platform.HotkeyLinks),
		},
		FinderConflict: platform.FinderConflict(a.symbolicHotkeysPath, settings),
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

// hotkeyView turns the system's answer for one shortcut into its state.
func hotkeyView(tab, spec string, id uint32) HotkeyView {
	view := HotkeyView{Tab: tab, Spec: spec, State: hotkeyPending}

	status, known := hotkeyStatus(id)
	if !known {
		return view
	}

	view.Status = status

	switch status {
	case hotkeyStatusOK:
		view.State = hotkeyRegistered
	case platform.HotkeyExistsStatus:
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
// through checkWelcome on that `shown`: on a stock Mac "Show Finder
// search window" keeps the default links shortcut, so that problem would
// otherwise raise the panel unasked on every launch. Asking from the
// page, instead of from startup, means the event never arrives before
// anyone listens.
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

// DismissWelcome hides the welcome for the rest of this run. The log
// line is what the verifiers read: the panel stays on screen either way,
// so nothing else tells Enter on the welcome apart from Enter lost.
func (a *App) DismissWelcome() {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.welcomeDismissed = true
	log.Printf("welcome: dismissed")
}

// OpenKeyboardSettings opens the pane where the Finder's shortcut can be
// turned off.
func (a *App) OpenKeyboardSettings() error {
	return a.open(platform.KeyboardSettingsURL)
}
