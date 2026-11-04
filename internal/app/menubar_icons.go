package app

import (
	"embed"
	"fmt"

	"github.com/drolosoft/hopto/internal/library"
	"github.com/drolosoft/hopto/internal/native"
)

// The drawings of the menu bar item, rendered by build/menubar/render.sh
// from the SVGs there: one PNG at 18 pixels and one at 36 for the
// Retina bar, named after the values of [settings] menubar_icon. The
// generic one has no file: it is the system's own symbol.
//
//go:embed menubar/*.png
var menubarFiles embed.FS

// menubarIconFor returns the drawing for a value of menubar_icon. All
// are templates (black with transparent holes, painted by the system in
// the bar's colour) except the frog in colour; the generic one is the
// empty Icon, which asks the native side for the system symbol. A value
// without files, which Validate never lets through, gets the symbol too.
func menubarIconFor(id string) native.Icon {
	if id == library.MenubarIconGeneric {
		return native.Icon{}
	}

	small, smallErr := menubarFiles.ReadFile(
		fmt.Sprintf("menubar/%s.png", id),
	)
	large, largeErr := menubarFiles.ReadFile(
		fmt.Sprintf("menubar/%s@2x.png", id),
	)
	if smallErr != nil || largeErr != nil {
		return native.Icon{}
	}

	return native.Icon{
		Small:    small,
		Large:    large,
		Template: id != library.MenubarIconColour,
	}
}
