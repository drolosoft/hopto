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
