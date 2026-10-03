package library

import "testing"

// The virtual chips of the apps tab cannot be taken by a user category.
func TestCheckCategoryRefusesVirtualIDs(t *testing.T) {
	reserved := []string{
		"favoritos", "favorites", "applications", "edge", "hidden",
	}

	for _, id := range reserved {
		problems := CheckCategory(Category{ID: id, Name: "X", Tab: TabApps})
		if len(problems) == 0 {
			t.Errorf("%q accepted as a category id", id)
		}
	}
}

// The screen constants are the words a library.toml carries, and the
// validator accepts every one of them.
func TestScreenValuesAreTheTOMLWords(t *testing.T) {
	want := map[string]string{
		ScreenLast:  "last",
		ScreenMouse: "mouse",
		ScreenMain:  "main",
	}

	for got, text := range want {
		if got != text {
			t.Errorf("screen constant %q, want %q", got, text)
		}

		if !screens[got] {
			t.Errorf("validator does not accept %q", got)
		}
	}
}
