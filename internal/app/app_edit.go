package app

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/drolosoft/hopto/internal/atomicfile"
	"github.com/drolosoft/hopto/internal/discover"
	"github.com/drolosoft/hopto/internal/icons"
	"github.com/drolosoft/hopto/internal/inspect"
	"github.com/drolosoft/hopto/internal/library"
	"github.com/drolosoft/hopto/internal/safehttp"
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

// iconFilePerm keeps the icons private like the rest of the data folder.
const iconFilePerm = 0o600

// LinkInput is what the editor sends for a link; the id is derived here.
type LinkInput struct {
	URL         string   `json:"url"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Keywords    []string `json:"keywords"`
	Icon        string   `json:"icon"`
}

// AppInput is what the editor sends for a hand-added app.
type AppInput struct {
	Path        string `json:"path"`
	BundleID    string `json:"bundleId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// SaveResult tells the editor how a save went: the id on success, the
// problems by field (keys the page translates), or the item this one
// duplicates. Problems is never nil.
type SaveResult struct {
	ID        string            `json:"id"`
	Problems  map[string]string `json:"problems"`
	Duplicate *Ref              `json:"duplicate"`
}

// LinkDraft is what the editor pre-fills after the user types a URL.
type LinkDraft struct {
	URL         string `json:"url"`
	Host        string `json:"host"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IconDataURL string `json:"iconDataUrl"`
	Insecure    bool   `json:"insecure"`
	Duplicate   *Ref   `json:"duplicate"`
	SameHost    []Ref  `json:"sameHost"`
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
			result = SaveResult{
				Problems:  map[string]string{},
				Duplicate: linkRef(duplicate),
			}

			return errAnswered
		}

		// An id is taken by either tab: a link and an app sharing one would
		// collide on icons/<id>.png and confuse CopyTarget, RevealInFinder
		// and iconURL.
		id := library.UniqueID(library.Slug(in.Name), func(candidate string) bool {
			return hasLinkID(*draft, candidate) || hasAppID(*draft, candidate)
		})

		link = linkFromInput(id, in)
		if problems := linkProblems(*draft, link); len(problems) > 0 {
			result = SaveResult{Problems: problems}
			return errAnswered
		}

		draft.Links = append(draft.Links, link)
		result = SaveResult{ID: id, Problems: map[string]string{}}

		return nil
	})
	if errors.Is(err, errAnswered) {
		return result, nil
	}
	if err != nil {
		return SaveResult{Problems: map[string]string{}}, err
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
			result = SaveResult{
				Problems:  map[string]string{},
				Duplicate: linkRef(duplicate),
			}

			return errAnswered
		}

		link = linkFromInput(id, in)
		if problems := linkProblems(*draft, link); len(problems) > 0 {
			result = SaveResult{Problems: problems}
			return errAnswered
		}

		draft.Links[index] = link
		result = SaveResult{ID: id, Problems: map[string]string{}}

		return nil
	})
	if errors.Is(err, errAnswered) {
		return result, nil
	}
	if err != nil {
		return SaveResult{Problems: map[string]string{}}, err
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
			result = SaveResult{Problems: map[string]string{}, Duplicate: twin}
			return errAnswered
		}

		// Same reasoning as AddLink: an id must not be taken on either tab.
		id := library.UniqueID(library.Slug(in.Name), func(candidate string) bool {
			return hasLinkID(*draft, candidate) || hasAppID(*draft, candidate)
		})

		app := appFromInput(id, in)
		if problems := a.appProblems(*draft, app); len(problems) > 0 {
			result = SaveResult{Problems: problems}
			return errAnswered
		}

		draft.Apps = append(draft.Apps, app)

		// The app just adopted by hand may be the one a [[hidden]] row
		// still names by its discovered id; appViews will never show
		// that id again, so the row would be a dead one otherwise, the
		// "Hidden" chip on an empty list the final review found.
		dropTwinHidden(draft, a.discover(draft.Settings), app)

		result = SaveResult{ID: id, Problems: map[string]string{}}

		return nil
	})
	if errors.Is(err, errAnswered) {
		return result, nil
	}
	if err != nil {
		return SaveResult{Problems: map[string]string{}}, err
	}

	return result, nil
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
		position := slices.IndexFunc(
			draft.Apps,
			func(entry library.AppEntry) bool { return entry.ID == id },
		)
		if position < 0 {
			return fmt.Errorf("%w: %s", errUnknownItem, id)
		}

		app := appFromInput(id, in)
		if problems := a.appProblems(*draft, app); len(problems) > 0 {
			result = SaveResult{Problems: problems}
			return errAnswered
		}

		draft.Apps[position] = app
		result = SaveResult{ID: id, Problems: map[string]string{}}

		return nil
	})
	if errors.Is(err, errAnswered) {
		return result, nil
	}
	if err != nil {
		return SaveResult{Problems: map[string]string{}}, err
	}

	return result, nil
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

// InspectURL prepares a draft from a typed URL: the scheme filled in,
// duplicates in the library, and what the page says about itself when it
// can be reached within inspect.Timeout. Network failures leave the URL
// and the host, which is enough to save.
func (a *App) InspectURL(raw string) (LinkDraft, error) {
	raw = withScheme(strings.TrimSpace(raw))

	if _, err := library.HostOf(raw); err != nil {
		return LinkDraft{SameHost: []Ref{}}, err
	}

	// HostOf also validates the port, but the editor shows the bare host:
	// the port is dial detail, not something the user typed to read back.
	parsed, _ := url.Parse(raw)
	draft := LinkDraft{
		URL:      raw,
		Host:     parsed.Hostname(),
		Insecure: strings.HasPrefix(raw, "http://"),
		SameHost: []Ref{},
	}

	a.reload()
	lib := a.library.Snapshot()

	if duplicate, ok := library.FindDuplicateLink(lib.Links, raw); ok {
		draft.Duplicate = linkRef(duplicate)
	}

	for _, neighbour := range library.LinksOnSameHost(lib.Links, raw) {
		draft.SameHost = append(draft.SameHost, *linkRef(neighbour))
	}

	ctx, cancel := context.WithTimeout(context.Background(), inspect.Timeout)
	defer cancel()

	// AllowHTTP only follows a redirect down to plain text when the typed
	// URL was already http://; an https:// URL must never be downgraded
	// while inspecting it.
	client := safehttp.NewClient(safehttp.Options{
		AllowPrivate: lib.Settings.AllowPrivateIconHosts,
		AllowHTTP:    strings.HasPrefix(raw, "http://"),
		Timeout:      inspect.Timeout,
	})
	if page, err := inspect.Fetch(ctx, client, raw); err == nil {
		draft.Name = page.SuggestedName()
		draft.Description = page.Description
	}

	iconCtx, cancelIcon := context.WithTimeout(
		context.Background(), icons.FetchTimeout,
	)
	defer cancelIcon()

	if png, err := a.newFetcher().Fetch(iconCtx, "", raw); err == nil {
		encoded := base64.StdEncoding.EncodeToString(png)
		draft.IconDataURL = "data:image/png;base64," + encoded
	}

	return draft, nil
}

// RefetchIcon downloads a link's icon again, or re-reads a hand-added
// app's bundle icon, writes it and returns the new page URL. Discovered
// apps are served live from their bundle and have nothing to refetch.
func (a *App) RefetchIcon(tab, id string) (string, error) {
	a.reload()

	if tab == tabLinks {
		link, ok := a.findLink(id)
		if !ok {
			return "", fmt.Errorf("%w: %s", errUnknownItem, id)
		}

		ctx, cancel := context.WithTimeout(
			context.Background(), icons.FetchTimeout,
		)
		defer cancel()

		png, err := a.newFetcher().Fetch(ctx, link.Icon, link.URL)
		if err != nil {
			return "", err
		}

		return a.writeIcon(id, png)
	}

	for _, app := range a.library.Snapshot().Apps {
		if app.ID != id {
			continue
		}

		bundle, err := discover.Inspect(app.Path)
		if err != nil || bundle.IconPath == "" {
			return "", fmt.Errorf("%w: %s", errNoIconFound, app.Path)
		}

		raw, ok := icons.SourcePNG(bundle.IconPath)
		if !ok {
			return "", fmt.Errorf("%w: %s", errNoIconFound, bundle.IconPath)
		}

		png, err := icons.Normalize(raw, icons.AppSide)
		if err != nil {
			return "", err
		}

		return a.writeIcon(id, png)
	}

	if _, ok := a.findDiscovered(id); ok {
		return "", fmt.Errorf("%w: %s", errLiveIcon, id)
	}

	return "", fmt.Errorf("%w: %s", errUnknownItem, id)
}

// fetchMissingIcons starts a background download for every link without
// an icon file, once per id per run: a site that has no icon is not asked
// again every time the panel opens.
func (a *App) fetchMissingIcons(links []library.Link) {
	// A broken library.toml is served from the last good snapshot, whose
	// ids may not be the user's any more; fetching for them would leave
	// the seed's icons behind in the real icons/ folder once the file is
	// fixed and those ids no longer mean the same entries.
	if a.library.Status().ReadOnly {
		return
	}

	for _, link := range links {
		if icons.FileURL(a.iconsDir(), link.ID) == "" {
			a.fetchIconLater(link.ID, link.Icon, link.URL)
		}
	}
}

// fetchIconLater downloads one icon in a goroutine and tells the page when
// it is on disk. The goroutine touches the icons folder and the window,
// never the library store; before writing, it checks under the lock that
// no later edit has moved the id on to a new generation (forgetFetch bumps
// it), and drops its answer rather than race a fresher fetch to disk.
func (a *App) fetchIconLater(id, hint, pageURL string) {
	if a.offline {
		return
	}

	a.mu.Lock()
	already := a.fetched[id]
	a.fetched[id] = true
	generation := a.fetchGeneration[id]
	a.mu.Unlock()

	if already {
		return
	}

	fetcher := a.newFetcher()

	a.background.Go(func() {
		ctx, cancel := context.WithTimeout(
			context.Background(), icons.FetchTimeout,
		)
		defer cancel()

		png, err := fetcher.Fetch(ctx, hint, pageURL)
		if err != nil {
			log.Printf("icon %s: %v", id, err)
			return
		}

		if !a.stillCurrent(id, generation) {
			return
		}

		if _, err := a.writeIcon(id, png); err != nil {
			log.Printf("icon %s: %v", id, err)
			return
		}

		if !a.stillCurrent(id, generation) {
			return
		}

		a.emit("icons", id)
	})
}

// stillCurrent reports whether generation is still the one an edit last
// started for id: a fetch that started before a later edit answers for the
// URL that edit replaced, and must not overwrite the newer fetch's result.
func (a *App) stillCurrent(id string, generation int) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	return a.fetchGeneration[id] == generation
}

// forgetFetch lets an id be fetched again (its URL or hint changed) and
// bumps its generation, so a fetch already in flight for the previous
// value drops its answer instead of writing over the new one.
func (a *App) forgetFetch(id string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	delete(a.fetched, id)
	a.fetchGeneration[id]++
}

// newFetcher builds a fetcher from the current settings; cheap enough to
// make per use, so a settings edit takes effect without a restart.
func (a *App) newFetcher() *icons.Fetcher {
	settings := a.library.Snapshot().Settings

	return icons.NewFetcher(
		settings.AllowPrivateIconHosts, settings.IconServices,
	)
}

// writeIcon saves a normalised PNG under icons/<id>.png and returns its
// page URL.
func (a *App) writeIcon(id string, png []byte) (string, error) {
	path := filepath.Join(a.iconsDir(), id+".png")
	if err := atomicfile.Write(path, png, iconFilePerm); err != nil {
		return "", err
	}

	return icons.FileURL(a.iconsDir(), id), nil
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

// tidyLinkInput trims every field, fills a missing scheme and drops empty
// keywords, so what the store validates is what the user meant.
func tidyLinkInput(in LinkInput) LinkInput {
	in.URL = withScheme(strings.TrimSpace(in.URL))
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.Category = strings.TrimSpace(in.Category)
	in.Icon = strings.TrimSpace(in.Icon)

	keywords := []string{}
	for _, keyword := range in.Keywords {
		if keyword = strings.TrimSpace(keyword); keyword != "" {
			keywords = append(keywords, keyword)
		}
	}
	in.Keywords = keywords

	return in
}

// tidyAppInput trims and cleans the path, so "/Applications/Safari.app/"
// matches what the validator wants.
func tidyAppInput(in AppInput) AppInput {
	in.Path = strings.TrimSpace(in.Path)
	if in.Path != "" {
		in.Path = filepath.Clean(in.Path)
	}

	in.BundleID = strings.TrimSpace(in.BundleID)
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.Category = strings.TrimSpace(in.Category)

	return in
}

// withScheme puts https:// in front of a bare host or domain path; a value
// with any scheme is left alone for the validator to judge.
func withScheme(raw string) string {
	if raw == "" || strings.Contains(raw, "://") {
		return raw
	}

	return "https://" + raw
}

// linkFromInput builds the entry the store validates.
func linkFromInput(id string, in LinkInput) library.Link {
	return library.Link{
		ID:          id,
		Name:        in.Name,
		Description: in.Description,
		URL:         in.URL,
		Category:    in.Category,
		Keywords:    in.Keywords,
		Icon:        in.Icon,
	}
}

// appFromInput builds the entry the store validates.
func appFromInput(id string, in AppInput) library.AppEntry {
	return library.AppEntry{
		ID:          id,
		Name:        in.Name,
		Description: in.Description,
		BundleID:    in.BundleID,
		Path:        in.Path,
		Category:    in.Category,
	}
}

// linkProblems maps the validator's findings to fields, adding the check
// the validator does against the whole library: the category exists on
// the links tab.
func linkProblems(lib library.Library, link library.Link) map[string]string {
	problems := problemMap(library.CheckLink(link))

	if !hasCategory(lib, tabLinks, link.Category) {
		problems["category"] = "category.unknown"
	}

	return problems
}

// appProblems is linkProblems for an app.
func (a *App) appProblems(
	lib library.Library, app library.AppEntry,
) map[string]string {
	problems := problemMap(library.CheckApp(app, a.home))

	if !hasCategory(lib, tabApps, app.Category) {
		problems["category"] = "category.unknown"
	}

	return problems
}

// problemMap keeps the first problem of each field.
func problemMap(problems []library.Problem) map[string]string {
	byField := map[string]string{}

	for _, problem := range problems {
		if _, seen := byField[problem.Field]; !seen {
			byField[problem.Field] = problem.Key
		}
	}

	return byField
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

// linkRef points the page at an existing link.
func linkRef(link library.Link) *Ref {
	return &Ref{Tab: tabLinks, ID: link.ID, Name: link.Name}
}
