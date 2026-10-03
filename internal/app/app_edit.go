// Saving, deleting and hiding the links and apps of the library, and the
// answer every save gives the editor.

package app

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/drolosoft/hopto/internal/discover"
	"github.com/drolosoft/hopto/internal/icons"
	"github.com/drolosoft/hopto/internal/library"
)

// The errors of editing that are not field problems.
var (
	errCategoryInUse = errors.New("the category still has items")
	errNotDiscovered = errors.New("only discovered apps can be hidden")
	errLiveIcon      = errors.New(
		"discovered apps serve their icon from the bundle",
	)
	errNoIconFound = icons.ErrNoIcon

	// errAnswered marks an Apply aborted on purpose because the closure
	// already worked out a Problems or Duplicate answer; the store must
	// not write anything, but the caller must not see it as a real error.
	errAnswered = errors.New("answered without writing")

	// errAlreadyAdopted answers HideApp on the id of a discovered app
	// that a hand-added entry already opens: the UI never offers it (a
	// twin never reaches the grid under its discovered id), and hiding
	// it would recreate the dead [[hidden]] row AddApp otherwise drops.
	errAlreadyAdopted = errors.New(
		"already added by hand, cannot be hidden under its discovered id",
	)
)

// SaveResult tells the editor how a save went: the id on success, the
// problems by field (keys the page translates), or the item this one
// duplicates. Problems is never nil.
type SaveResult struct {
	ID        string            `json:"id"`
	Problems  map[string]string `json:"problems"`
	Duplicate *Ref              `json:"duplicate"`
}

// AddLink validates and stores a new link, then fetches its icon in the
// background. Field problems and duplicates are answers, not errors. The
// id, the duplicate check and the field checks all run against the draft
// inside Apply, under the store's own lock: two adds racing on the same
// name must not compute the same id.
func (a *App) AddLink(in LinkInput) (SaveResult, error) {
	a.reload()
	in = tidyLinkInput(in)

	var result SaveResult
	var link library.Link

	err := a.library.Apply(func(draft *library.Library) error {
		if duplicate, ok := library.FindDuplicateLink(draft.Links, in.URL); ok {
			result = duplicateOf(linkRef(duplicate))
			return errAnswered
		}

		id := freeID(*draft, in.Name)

		link = linkFromInput(id, in)
		if problems := linkProblems(*draft, link); len(problems) > 0 {
			result = SaveResult{Problems: problems}
			return errAnswered
		}

		draft.Links = append(draft.Links, link)
		result = savedAs(id)

		return nil
	})
	if err != nil {
		return saveOutcome(result, err)
	}

	a.fetchIconLater(result.ID, link.Icon, link.URL)

	return result, nil
}

// UpdateLink replaces the fields of a link; the id never changes. Like
// AddLink, the duplicate and field checks run against the draft inside
// Apply, under the store's own lock: a link added to the same URL while
// this update runs must be seen, not a snapshot taken before it. The
// editor has no icon field, so an empty hint keeps the one the link had.
func (a *App) UpdateLink(id string, in LinkInput) (SaveResult, error) {
	a.reload()
	in = tidyLinkInput(in)

	var result SaveResult
	var previous, link library.Link

	err := a.library.Apply(func(draft *library.Library) error {
		index := slices.IndexFunc(draft.Links, func(entry library.Link) bool {
			return entry.ID == id
		})
		if index < 0 {
			return fmt.Errorf("%w: %s", errUnknownItem, id)
		}

		previous = draft.Links[index]
		if in.Icon == "" {
			in.Icon = previous.Icon
		}

		others := slices.Delete(slices.Clone(draft.Links), index, index+1)
		if duplicate, ok := library.FindDuplicateLink(others, in.URL); ok {
			result = duplicateOf(linkRef(duplicate))
			return errAnswered
		}

		link = linkFromInput(id, in)
		if problems := linkProblems(*draft, link); len(problems) > 0 {
			result = SaveResult{Problems: problems}
			return errAnswered
		}

		draft.Links[index] = link
		result = savedAs(id)

		return nil
	})
	if err != nil {
		return saveOutcome(result, err)
	}

	if link.URL != previous.URL || link.Icon != previous.Icon {
		a.forgetFetch(id)
		a.fetchIconLater(id, link.Icon, link.URL)
	}

	return result, nil
}

// DeleteLink removes a link, its usage and its icon file.
func (a *App) DeleteLink(id string) error {
	if err := a.removeEntry(id, tabLinks); err != nil {
		return err
	}

	a.forgetItem(tabLinks, id)

	return nil
}

// AddApp stores a hand-added app. A path without a name takes the bundle's
// folder name, as the Finder shows it. The id and the field checks run
// against the draft inside Apply, under the store's own lock, for the
// same reason as AddLink: two adds racing on the same name must not
// compute the same id.
func (a *App) AddApp(in AppInput) (SaveResult, error) {
	a.reload()
	in = tidyAppInput(in)

	if in.Name == "" && in.Path != "" {
		bundle, _ := discover.Inspect(in.Path)
		in.Name = bundle.Name
	}

	var result SaveResult

	err := a.library.Apply(func(draft *library.Library) error {
		// The page checked PickApp's snapshot, which a second editor
		// or a hand edit may have outrun: the same rule runs again
		// here, on the file as it is now, under the store's lock. Only
		// an entry added by hand blocks; a found app is adopted.
		twin := handAddedDuplicate(*draft, in.Path, in.BundleID)
		if twin != nil {
			result = duplicateOf(twin)
			return errAnswered
		}

		id := freeID(*draft, in.Name)

		app := appFromInput(id, in)
		if problems := a.appProblems(*draft, app); len(problems) > 0 {
			result = SaveResult{Problems: problems}
			return errAnswered
		}

		draft.Apps = append(draft.Apps, app)

		// The app just adopted by hand may be the one a [[hidden]] row
		// still names by its discovered id; appViews will never show
		// that id again, so the row would be a dead one otherwise.
		dropTwinHidden(draft, a.discover(draft.Settings), app)

		result = savedAs(id)

		return nil
	})

	return saveOutcome(result, err)
}

// dropTwinHidden removes the [[hidden]] row of a discovered app once the
// user has just adopted it by hand under app's id.
func dropTwinHidden(
	draft *library.Library, discovered []discover.App, app library.AppEntry,
) {
	for _, found := range discovered {
		if !sameApp(app.Path, app.BundleID, found.Path, found.BundleID) {
			continue
		}

		draft.Hidden = slices.DeleteFunc(
			draft.Hidden,
			func(hidden library.Hidden) bool { return hidden.ID == found.ID },
		)
	}
}

// UpdateApp replaces the fields of a hand-added app, checking them inside
// Apply for the same reason as UpdateLink.
func (a *App) UpdateApp(id string, in AppInput) (SaveResult, error) {
	a.reload()
	in = tidyAppInput(in)

	var result SaveResult

	err := a.library.Apply(func(draft *library.Library) error {
		index := slices.IndexFunc(draft.Apps, func(entry library.AppEntry) bool {
			return entry.ID == id
		})
		if index < 0 {
			return fmt.Errorf("%w: %s", errUnknownItem, id)
		}

		app := appFromInput(id, in)
		if problems := a.appProblems(*draft, app); len(problems) > 0 {
			result = SaveResult{Problems: problems}
			return errAnswered
		}

		draft.Apps[index] = app
		result = savedAs(id)

		return nil
	})

	return saveOutcome(result, err)
}

// DeleteApp removes a hand-added app, its usage and its icon file.
func (a *App) DeleteApp(id string) error {
	if err := a.removeEntry(id, tabApps); err != nil {
		return err
	}

	a.forgetItem(tabApps, id)

	return nil
}

// HideApp keeps a discovered app out of the list. Hiding twice is fine;
// hiding the discovered id of an app already adopted by hand is refused,
// since it is unreachable from the UI (a twin never reaches the grid
// under that id) and would only recreate the dead row AddApp drops.
func (a *App) HideApp(id string) error {
	isDiscovered := strings.HasPrefix(id, discover.PrefixApplications) ||
		strings.HasPrefix(id, discover.PrefixEdge)
	if !isDiscovered {
		return fmt.Errorf("%w: %s", errNotDiscovered, id)
	}

	return a.library.Apply(func(draft *library.Library) error {
		twins := adoptedTwins(draft.Apps)
		for _, found := range a.discover(draft.Settings) {
			if found.ID == id && isAdopted(twins, found.Path, found.BundleID) {
				return fmt.Errorf("%w: %s", errAlreadyAdopted, id)
			}
		}

		for _, hidden := range draft.Hidden {
			if hidden.ID == id {
				return nil
			}
		}

		draft.Hidden = append(draft.Hidden, library.Hidden{ID: id})
		return nil
	})
}

// UnhideApp puts a hidden app back.
func (a *App) UnhideApp(id string) error {
	return a.library.Apply(func(draft *library.Library) error {
		draft.Hidden = slices.DeleteFunc(
			draft.Hidden,
			func(hidden library.Hidden) bool { return hidden.ID == id },
		)
		return nil
	})
}

// removeEntry deletes a link or an app from the library by id.
func (a *App) removeEntry(id, tab string) error {
	return a.library.Apply(func(draft *library.Library) error {
		before := len(draft.Links) + len(draft.Apps)

		if tab == tabLinks {
			draft.Links = slices.DeleteFunc(
				draft.Links,
				func(link library.Link) bool { return link.ID == id },
			)
		} else {
			draft.Apps = slices.DeleteFunc(
				draft.Apps,
				func(app library.AppEntry) bool { return app.ID == id },
			)
		}

		if len(draft.Links)+len(draft.Apps) == before {
			return fmt.Errorf("%w: %s", errUnknownItem, id)
		}

		return nil
	})
}

// forgetItem drops the usage and the icon of a deleted item, so a later
// item with the same id starts clean. Failures are logged: the item is
// already gone from the library.
func (a *App) forgetItem(tab, id string) {
	if err := a.usage.Forget(tab + ":" + id); err != nil {
		log.Printf("usage: %v", err)
	}

	// forgetFetch bumps the id's generation before the icon file is
	// removed, so a fetch already in flight for the deleted item finds
	// itself stale and drops its answer instead of writing an orphan
	// icons/<id>.png after this function returns.
	a.forgetFetch(id)

	iconPath := filepath.Join(a.iconsDir(), id+".png")
	err := os.Remove(iconPath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Printf("icon %s: %v", id, err)
	}
}

// hasLinkID reports whether an id is already taken on the links tab.
func hasLinkID(lib library.Library, id string) bool {
	return slices.ContainsFunc(lib.Links, func(link library.Link) bool {
		return link.ID == id
	})
}

// hasAppID reports whether an id is already taken on the apps tab.
func hasAppID(lib library.Library, id string) bool {
	return slices.ContainsFunc(lib.Apps, func(app library.AppEntry) bool {
		return app.ID == id
	})
}

// freeID is the id a new entry called name gets: its slug, stepped aside
// from every id on either tab. A link and an app sharing one would
// collide on icons/<id>.png and confuse CopyTarget, RevealInFinder and
// iconURL. AddLink and AddApp call it inside Apply, on the draft.
func freeID(lib library.Library, name string) string {
	return library.UniqueID(library.Slug(name), func(candidate string) bool {
		return hasLinkID(lib, candidate) || hasAppID(lib, candidate)
	})
}

// savedAs is the answer to a save that went through under id.
func savedAs(id string) SaveResult {
	return SaveResult{ID: id, Problems: map[string]string{}}
}

// duplicateOf is the answer to a save refused because ref already is
// the same item.
func duplicateOf(ref *Ref) SaveResult {
	return SaveResult{Problems: map[string]string{}, Duplicate: ref}
}

// saveOutcome turns what Apply returned into what the four saves return,
// so the rule is written once: errAnswered means the closure already
// answered in result (problems or a duplicate) and is no error to the
// page; any other error is a real one, with an empty answer; nil is the
// saved result.
func saveOutcome(result SaveResult, err error) (SaveResult, error) {
	if errors.Is(err, errAnswered) {
		return result, nil
	}

	if err != nil {
		return SaveResult{Problems: map[string]string{}}, err
	}

	return result, nil
}

// linkRef points the page at an existing link.
func linkRef(link library.Link) *Ref {
	return &Ref{Tab: tabLinks, ID: link.ID, Name: link.Name}
}
