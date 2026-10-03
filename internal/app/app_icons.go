// The link inspector and the icon work: reading what a typed URL says
// about itself, fetching a site's icon or re-reading a bundle's, and
// writing them under icons/.

package app

import (
	"context"
	"encoding/base64"
	"fmt"
	"log"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/drolosoft/hopto/internal/atomicfile"
	"github.com/drolosoft/hopto/internal/discover"
	"github.com/drolosoft/hopto/internal/icons"
	"github.com/drolosoft/hopto/internal/inspect"
	"github.com/drolosoft/hopto/internal/library"
	"github.com/drolosoft/hopto/internal/safehttp"
)

// iconFilePerm keeps the icons private like the rest of the data folder.
const iconFilePerm = 0o600

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

	page, ok := fetchPageInfo(raw, lib.Settings.AllowPrivateIconHosts)
	if ok {
		draft.Name = page.SuggestedName()
		draft.Description = page.Description
	}

	draft.IconDataURL = a.fetchIconDataURL(raw)

	return draft, nil
}

// fetchPageInfo reads what the page at raw says about itself, within
// inspect.Timeout; ok is false when it could not be reached or read.
func fetchPageInfo(raw string, allowPrivate bool) (inspect.Page, bool) {
	ctx, cancel := context.WithTimeout(context.Background(), inspect.Timeout)
	defer cancel()

	// AllowHTTP only follows a redirect down to plain text when the typed
	// URL was already http://; an https:// URL must never be downgraded
	// while inspecting it.
	client := safehttp.NewClient(safehttp.Options{
		AllowPrivate: allowPrivate,
		AllowHTTP:    strings.HasPrefix(raw, "http://"),
		Timeout:      inspect.Timeout,
	})

	page, err := inspect.Fetch(ctx, client, raw)

	return page, err == nil
}

// fetchIconDataURL is the site's icon as a data URL for the editor's
// preview (the draft has no id yet, so no icon file), or "" when none
// came within icons.FetchTimeout.
func (a *App) fetchIconDataURL(raw string) string {
	ctx, cancel := context.WithTimeout(
		context.Background(), icons.FetchTimeout,
	)
	defer cancel()

	png, err := a.newFetcher().Fetch(ctx, "", raw)
	if err != nil {
		return ""
	}

	encoded := base64.StdEncoding.EncodeToString(png)

	return "data:image/png;base64," + encoded
}

// RefetchIcon downloads a link's icon again, or re-reads a hand-added
// app's bundle icon, writes it and returns the new page URL. Discovered
// apps are served live from their bundle and have nothing to refetch.
func (a *App) RefetchIcon(tab, id string) (string, error) {
	a.reload()

	if tab == tabLinks {
		return a.refetchLinkIcon(id)
	}

	if app, ok := a.libraryApp(id); ok {
		return a.refetchAppIcon(app)
	}

	if _, ok := a.findDiscovered(id); ok {
		return "", fmt.Errorf("%w: %s", errLiveIcon, id)
	}

	return "", fmt.Errorf("%w: %s", errUnknownItem, id)
}

// refetchLinkIcon downloads the icon of the link id again and writes it.
func (a *App) refetchLinkIcon(id string) (string, error) {
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

// refetchAppIcon re-reads the icon of a hand-added app's bundle and
// writes it as the app's own.
func (a *App) refetchAppIcon(app library.AppEntry) (string, error) {
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

	return a.writeIcon(app.ID, png)
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
