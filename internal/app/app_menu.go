package app

import (
	"log"

	"github.com/drolosoft/hopto/internal/library"
	"github.com/drolosoft/hopto/internal/native"
	"github.com/drolosoft/hopto/internal/platform"
)

// The tags of the menu entries; the native side hands them back on a
// click. A rule is a native.MenuItem with Separator set, not a tag.
// The entries of the Icon submenu take the tags from menuIconFirst on,
// one per value of library.MenubarIcons, in its order.
const (
	menuOpen      = 1
	menuHelp      = 2
	menuEdit      = 3
	menuLogin     = 4
	menuQuit      = 5
	menuIconFirst = 11
)

// menuBar is the menu bar item (the tray icon on Windows), behind an
// interface so the tests can check the menu without the native one.
type menuBar interface {
	Install(items []native.MenuItem)
	SetChecked(tag int, on bool)
	SetIcon(icon native.Icon)
}

// menuLabels are the texts of the menu. The page's i18n.js cannot reach
// a native menu, so the two languages live here too. Icons holds the
// name of each drawing of the Icon submenu, by its setting value.
type menuLabels struct {
	Open, Help, Edit, Login, Icon, Quit string
	Icons                               map[string]string
}

// menuLabelsFor picks the texts for a resolved language.
func menuLabelsFor(language string) menuLabels {
	if language == platform.LanguageSpanish {
		return menuLabels{
			Open:  "Abrir hopto",
			Help:  "Ayuda",
			Edit:  "Editar library.toml",
			Login: "Arrancar al iniciar sesión",
			Icon:  "Icono",
			Quit:  "Salir de hopto",
			Icons: map[string]string{
				library.MenubarIconFace:      "Cara",
				library.MenubarIconKey:       "Tecla",
				library.MenubarIconIsometric: "Isométrica",
				library.MenubarIconPeeking:   "Asomando",
				library.MenubarIconOutline:   "Trazo",
				library.MenubarIconColour:    "En color",
				library.MenubarIconGeneric:   "Genérico",
			},
		}
	}

	return menuLabels{
		Open:  "Open hopto",
		Help:  "Help",
		Edit:  "Edit library.toml",
		Login: "Open at login",
		Icon:  "Icon",
		Quit:  "Quit hopto",
		Icons: map[string]string{
			library.MenubarIconFace:      "Face",
			library.MenubarIconKey:       "Key",
			library.MenubarIconIsometric: "Isometric",
			library.MenubarIconPeeking:   "Peeking",
			library.MenubarIconOutline:   "Outline",
			library.MenubarIconColour:    "In colour",
			library.MenubarIconGeneric:   "Generic",
		},
	}
}

// menuItems is the menu: Open, Help, Edit and Login, the Icon submenu
// on macOS, then a rule and Quit, which the system's own menus also
// keep apart.
func (a *App) menuItems() []native.MenuItem {
	settings := a.library.Snapshot().Settings
	labels := menuLabelsFor(platform.LanguageFor(settings.Language, a.language))

	items := []native.MenuItem{
		{Label: labels.Open, Tag: menuOpen},
		{Label: labels.Help, Tag: menuHelp},
		{Label: labels.Edit, Tag: menuEdit},
		{Label: labels.Login, Tag: menuLogin, Checked: a.login.Enabled()},
	}

	// The tray on Windows shows the app icon like every other tray icon,
	// so a choice there would change nothing.
	if menuOffersIcons {
		items = append(items, native.MenuItem{
			Label:    labels.Icon,
			Children: menuIconItems(labels, settings.MenubarIcon),
		})
	}

	return append(items,
		native.MenuItem{Separator: true},
		native.MenuItem{Label: labels.Quit, Tag: menuQuit},
	)
}

// menuOffersIcons says whether the menu has the Icon submenu: only
// where the item draws what the setting says.
const menuOffersIcons = platform.Name != "windows"

// menuIconItems is the Icon submenu: one entry per drawing, in the
// order of library.MenubarIcons, each next to its drawing, with the
// chosen one ticked.
func menuIconItems(labels menuLabels, chosen string) []native.MenuItem {
	items := make([]native.MenuItem, 0, len(library.MenubarIcons))
	for index, id := range library.MenubarIcons {
		icon := menubarIconFor(id)
		items = append(items, native.MenuItem{
			Label:   labels.Icons[id],
			Tag:     menuIconFirst + index,
			Checked: id == chosen,
			Icon:    &icon,
		})
	}

	return items
}

// menuIconFromTag is the setting value behind a tag of the Icon
// submenu, and false for any other tag.
func menuIconFromTag(tag int) (string, bool) {
	index := tag - menuIconFirst
	if index < 0 || index >= len(library.MenubarIcons) {
		return "", false
	}

	return library.MenubarIcons[index], true
}

// installMenu puts the item in the menu bar with the drawing the
// settings name; its clicks reach menuAction through the hooks startup
// hands to native.Start.
func (a *App) installMenu() {
	a.menu.Install(a.menuItems())
	a.menu.SetIcon(menubarIconFor(a.library.Snapshot().Settings.MenubarIcon))
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
		if id, ok := menuIconFromTag(tag); ok {
			a.chooseMenubarIcon(id)
			return
		}

		log.Printf("menu: unknown entry %d", tag)
	}
}

// chooseMenubarIcon saves a drawing picked from the Icon submenu and
// shows it. A refused save (the file broken by a hand edit) changes
// nothing on screen: the ticks keep telling what the file says.
func (a *App) chooseMenubarIcon(id string) {
	err := a.library.Apply(func(lib *library.Library) error {
		lib.Settings.MenubarIcon = id

		return nil
	})
	if err != nil {
		log.Printf("menu: icon %s: %v", id, err)
		return
	}

	for index, other := range library.MenubarIcons {
		a.menu.SetChecked(menuIconFirst+index, other == id)
	}

	a.menu.SetIcon(menubarIconFor(id))
	log.Printf("menu: icon %s", id)
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
