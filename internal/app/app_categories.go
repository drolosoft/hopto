// The category chips of each tab: adding, renaming and deleting them.

package app

import (
	"fmt"
	"slices"
	"strings"

	"github.com/drolosoft/hopto/internal/library"
)

// AddCategory creates a chip on a tab. The id comes from the name and
// steps aside from existing and reserved ids ("Favoritos" → favoritos-2).
// It is worked out against the draft inside Apply, under the store's own
// lock, so two adds racing on the same name cannot compute the same id.
func (a *App) AddCategory(tab, name string) (CategoryView, error) {
	if tab != tabApps && tab != tabLinks {
		return CategoryView{}, fmt.Errorf("%w: tab %q", errUnknownItem, tab)
	}

	a.reload()
	name = strings.TrimSpace(name)

	var category library.Category

	err := a.library.Apply(func(draft *library.Library) error {
		taken := func(candidate string) bool {
			probe := library.Category{ID: candidate, Name: "x", Tab: tab}
			if library.CheckCategory(probe) != nil {
				return true
			}

			return slices.ContainsFunc(
				draft.Categories,
				func(chip library.Category) bool { return chip.ID == candidate },
			)
		}

		category = library.Category{
			ID:   library.UniqueID(library.Slug(name), taken),
			Name: name,
			Tab:  tab,
		}
		draft.Categories = append(draft.Categories, category)

		return nil
	})
	if err != nil {
		return CategoryView{}, err
	}

	return CategoryView{ID: category.ID, Name: category.Name, Tab: tab}, nil
}

// RenameCategory changes the name of a chip; the id stays.
func (a *App) RenameCategory(tab, id, name string) error {
	name = strings.TrimSpace(name)

	return a.library.Apply(func(draft *library.Library) error {
		for index := range draft.Categories {
			sameChip := draft.Categories[index].ID == id &&
				draft.Categories[index].Tab == tab
			if sameChip {
				draft.Categories[index].Name = name
				return nil
			}
		}

		return fmt.Errorf("%w: category %s", errUnknownItem, id)
	})
}

// DeleteCategory removes an empty chip; one that still has items is kept.
func (a *App) DeleteCategory(tab, id string) error {
	return a.library.Apply(func(draft *library.Library) error {
		index := slices.IndexFunc(
			draft.Categories,
			func(category library.Category) bool {
				return category.ID == id && category.Tab == tab
			},
		)
		if index < 0 {
			return fmt.Errorf("%w: category %s", errUnknownItem, id)
		}

		inUse := slices.ContainsFunc(draft.Links, func(link library.Link) bool {
			return link.Category == id
		}) || slices.ContainsFunc(draft.Apps, func(app library.AppEntry) bool {
			return app.Category == id
		})
		if inUse {
			return fmt.Errorf("%w: %s", errCategoryInUse, id)
		}

		draft.Categories = slices.Delete(draft.Categories, index, index+1)
		return nil
	})
}

// hasCategory reports whether a chip exists on a tab.
func hasCategory(lib library.Library, tab, id string) bool {
	return slices.ContainsFunc(
		lib.Categories,
		func(category library.Category) bool {
			return category.ID == id && category.Tab == tab
		},
	)
}
