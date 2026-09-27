package main

import "log"

// The tags of the menu entries; the native side hands them back on a
// click. menuSeparator is a rule, not an entry.
const (
	menuSeparator = 0
	menuOpen      = 1
	menuHelp      = 2
	menuEdit      = 3
	menuLogin     = 4
	menuQuit      = 5
)

// onMenu is what a click in the menu ends up in. The native callback
// runs on the main thread and only starts a goroutine with it, like
// onHotkey; installMenu points it at the App.
var onMenu = func(tag int) {}

// menuItem is one line of the menu as the native side draws it.
type menuItem struct {
	Title   string
	Tag     int
	Checked bool
}

// menuBar is the status item, behind an interface so the tests can
// check the menu without AppKit.
type menuBar interface {
	Install(items []menuItem)
	SetChecked(tag int, on bool)
}

// menuLabels are the texts of the menu. The page's i18n.js cannot reach
// a native menu, so the two languages live here too.
type menuLabels struct {
	Open, Help, Edit, Login, Quit string
}

// menuLabelsFor picks the texts for a resolved language.
func menuLabelsFor(language string) menuLabels {
	if language == languageSpanish {
		return menuLabels{
			Open:  "Abrir hopto",
			Help:  "Ayuda",
			Edit:  "Editar library.toml",
			Login: "Arrancar al iniciar sesión",
			Quit:  "Salir de hopto",
		}
	}

	return menuLabels{
		Open:  "Open hopto",
		Help:  "Help",
		Edit:  "Edit library.toml",
		Login: "Open at login",
		Quit:  "Quit hopto",
	}
}

// menuItems is the menu in the order the spec lists it, with Quit
// after a rule as every macOS menu has it.
func (a *App) menuItems() []menuItem {
	setting := a.library.Snapshot().Settings.Language
	labels := menuLabelsFor(languageFor(setting, a.language))

	return []menuItem{
		{Title: labels.Open, Tag: menuOpen},
		{Title: labels.Help, Tag: menuHelp},
		{Title: labels.Edit, Tag: menuEdit},
		{Title: labels.Login, Tag: menuLogin, Checked: a.login.Enabled()},
		{Tag: menuSeparator},
		{Title: labels.Quit, Tag: menuQuit},
	}
}

// installMenu puts the item in the menu bar and routes its clicks here.
func (a *App) installMenu() {
	onMenu = a.menuAction
	a.menu.Install(a.menuItems())
}

// menuAction runs one entry. It is called from a goroutine, so it may
// use the window like any bound method.
func (a *App) menuAction(tag int) {
	switch tag {
	case menuOpen:
		a.showPanel(tabApps)
	case menuHelp:
		// Only once the panel is really up: under a dialog the help
		// would land on a page nobody can reach.
		if a.showPanel(tabApps) {
			a.emit("help", true)
		}
	case menuEdit:
		if err := a.EditLibrary(); err != nil {
			log.Printf("menu: edit library: %v", err)
		}
	case menuLogin:
		a.toggleLoginItem()
	case menuQuit:
		log.Printf("menu: quit")
		a.window.Quit()
	default:
		log.Printf("menu: unknown entry %d", tag)
	}
}

// showPanel brings the panel up on a tab. Unlike the shortcuts it never
// hides it: a menu entry called Open that closed the panel would read
// as broken. With the panel already up it only brings it forward. With
// a dialog on top it does nothing and says so, for Help to stay out of
// the way too.
func (a *App) showPanel(tab string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.dialogOpen {
		return false
	}

	if a.visible {
		a.window.Activate()
		return true
	}

	a.showLocked(tab)

	return true
}

// toggleLoginItem flips the LaunchAgent and ticks the entry from what is
// on disk afterwards, so a refusal (outside a bundle, a full disk)
// leaves the tick telling the truth.
func (a *App) toggleLoginItem() {
	var err error
	if a.login.Enabled() {
		err = a.login.Disable()
	} else {
		err = a.login.Enable()
	}

	if err != nil {
		log.Printf("menu: open at login: %v", err)
	}

	a.menu.SetChecked(menuLogin, a.login.Enabled())
}
