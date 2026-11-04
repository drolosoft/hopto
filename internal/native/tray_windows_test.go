//go:build windows

package native

import (
	"testing"
)

// The menu functions this test reads back with; the ones hopto itself
// only ever writes through are in native_windows.go.
var (
	procGetMenuItemCount = user32.NewProc("GetMenuItemCount")
	procGetSubMenu       = user32.NewProc("GetSubMenu")
	procGetMenuState     = user32.NewProc("GetMenuState")
)

// How GetMenuState names an entry (winuser.h): by its id, or by its
// position in the menu.
const (
	mfByCommand  = 0x0000
	mfByPosition = 0x0400
)

// The tray menu is built from the entries the launcher hands over: an
// entry with children becomes a submenu with those entries, ticks are
// kept inside it, and a rule stays a rule. Read back through the same
// Win32 API the tray draws with, on a real popup menu.
func TestAppendMenuItemsBuildsSubmenus(t *testing.T) {
	menu, _, _ := procCreatePopupMenu.Call()
	if menu == 0 {
		t.Fatal("CreatePopupMenu refused")
	}
	defer procDestroyMenu.Call(menu) //nolint:errcheck // a test's cleanup

	appendMenuItems(menu, []MenuItem{
		{Label: "Open", Tag: 1},
		{Label: "Icon", Children: []MenuItem{
			{Label: "Key", Tag: 11, Checked: true},
			{Label: "Face", Tag: 12},
		}},
		{Separator: true},
		{Label: "Quit", Tag: 5},
	})

	if count, _, _ := procGetMenuItemCount.Call(menu); count != 4 {
		t.Fatalf("top entries = %d, want 4", int32(count))
	}

	submenu, _, _ := procGetSubMenu.Call(menu, 1)
	if submenu == 0 {
		t.Fatal("the second entry opens no submenu")
	}

	if count, _, _ := procGetMenuItemCount.Call(submenu); count != 2 {
		t.Fatalf("submenu entries = %d, want 2", int32(count))
	}

	// GetMenuState by command finds the entry wherever it is; the tick
	// set on the child must have reached it.
	state, _, _ := procGetMenuState.Call(menu, 11, mfByCommand)
	if state&mfChecked == 0 {
		t.Errorf("Key is not ticked: state %#x", state)
	}

	state, _, _ = procGetMenuState.Call(menu, 12, mfByCommand)
	if state&mfChecked != 0 {
		t.Errorf("Face is ticked: state %#x", state)
	}

	// Position 2 of the top menu is the rule.
	state, _, _ = procGetMenuState.Call(menu, 2, mfByPosition)
	if state&mfSeparator == 0 {
		t.Errorf("the third entry is not a rule: state %#x", state)
	}
}
