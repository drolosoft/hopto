package app

import (
	"log"

	"github.com/drolosoft/hopto/internal/native"
	"github.com/drolosoft/hopto/internal/platform"
)

// The tags of the menu entries; the native side hands them back on a
// click. A rule is a native.MenuItem with Separator set, not a tag.
const (
	menuOpen  = 1
	menuHelp  = 2
	menuEdit  = 3
	menuLogin = 4
	menuQuit  = 5
)

// menuBar is the menu bar item (the tray icon on Windows), behind an
// interface so the tests can check the menu without the native one.
type menuBar interface {
	Install(items []native.MenuItem)
	SetChecked(tag int, on bool)
}

// menuLabels are the texts of the menu. The page's i18n.js cannot reach
// a native menu, so the two languages live here too.
type menuLabels struct {
	Open, Help, Edit, Login, Quit string
}

// menuLabelsFor picks the texts for a resolved language.
func menuLabelsFor(language string) menuLabels {
	if language == platform.LanguageSpanish {
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

// menuItems is the menu: Open, Help, Edit and Login, then a rule and
// Quit, which the system's own menus also keep apart.
func (a *App) menuItems() []native.MenuItem {
	setting := a.library.Snapshot().Settings.Language
	labels := menuLabelsFor(platform.LanguageFor(setting, a.language))

	return []native.MenuItem{
		{Label: labels.Open, Tag: menuOpen},
		{Label: labels.Help, Tag: menuHelp},
		{Label: labels.Edit, Tag: menuEdit},
		{Label: labels.Login, Tag: menuLogin, Checked: a.login.Enabled()},
		{Separator: true},
		{Label: labels.Quit, Tag: menuQuit},
	}
}

// installMenu puts the item in the menu bar; its clicks reach menuAction
// through the hooks startup hands to native.Start.
func (a *App) installMenu() {
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

// toggleLoginItem flips the login switch (a LaunchAgent on macOS, the Run
// key on Windows) and ticks the entry from what is there afterwards, so a
// refusal (outside a bundle, a full disk) leaves the tick telling the
// truth.
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
