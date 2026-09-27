package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/drolosoft/hopto/internal/discover"
	"github.com/drolosoft/hopto/internal/library"
	"github.com/drolosoft/hopto/internal/usage"
)

// The errors the page may see from an opening; validation problems never
// come this way.
var (
	errUnknownItem        = errors.New("unknown item")
	errNotInstalled       = errors.New("the app is not on disk")
	errNoSecondaryBrowser = errors.New("no secondary_browser in the settings")
	errNoPath             = errors.New("the item has no path to reveal")
)

// Launch opens an app through `open`, counts the opening and hides the
// panel. A hand-added app with a bundle id opens with -b, which survives
// the .app being moved; anything else opens by path.
func (a *App) Launch(id string) error {
	target, err := a.appTarget(id)
	if err != nil {
		return err
	}

	if err := a.open(target...); err != nil {
		log.Printf("open %v: %v", target, err)
		return err
	}

	log.Printf("launched %s: %v", id, target)
	a.recordOpen(tabApps + ":" + id)
	a.Hide()

	return nil
}

// OpenLink opens a link in the default browser. Only library ids are
// accepted: the page never passes a URL, so nothing outside the library
// can be opened.
func (a *App) OpenLink(id string) error {
	link, ok := a.findLink(id)
	if !ok {
		return fmt.Errorf("%w: %s", errUnknownItem, id)
	}

	return a.openLink(link, []string{link.URL})
}

// OpenLinkWith opens a link in the secondary browser of the settings.
func (a *App) OpenLinkWith(id string) error {
	link, ok := a.findLink(id)
	if !ok {
		return fmt.Errorf("%w: %s", errUnknownItem, id)
	}

	browser := a.library.Snapshot().Settings.SecondaryBrowser
	if browser == "" {
		return errNoSecondaryBrowser
	}

	return a.openLink(link, []string{"-b", browser, link.URL})
}

// openLink runs `open` with the arguments, counts and hides.
func (a *App) openLink(link library.Link, args []string) error {
	if err := a.open(args...); err != nil {
		log.Printf("open %s: %v", link.URL, err)
		return err
	}

	log.Printf("opened link %s: %s", link.ID, link.URL)
	a.recordOpen(tabLinks + ":" + link.ID)
	a.Hide()

	return nil
}

// CopyTarget puts the URL of a link, or the path (or bundle id) of an app,
// on the clipboard.
func (a *App) CopyTarget(id string) error {
	if link, ok := a.findLink(id); ok {
		return a.window.SetClipboard(link.URL)
	}

	target, err := a.appTarget(id)
	if err != nil {
		return err
	}

	// The last argument of an open command is the thing itself.
	return a.window.SetClipboard(target[len(target)-1])
}

// RevealInFinder shows an app's bundle in the Finder.
func (a *App) RevealInFinder(id string) error {
	if _, ok := a.findLink(id); ok {
		return errNoPath
	}

	path, err := a.appPath(id)
	if err != nil {
		return err
	}

	return a.open("-R", path)
}

// RevealLibrary shows library.toml in the Finder.
func (a *App) RevealLibrary() error {
	return a.open("-R", a.library.Path())
}

// EditLibrary opens library.toml in the default text editor.
func (a *App) EditLibrary() error {
	return a.open("-t", a.library.Path())
}

// ToggleFavorite marks or unmarks an item ("links:mdn", "apps:app-safari")
// and returns whether it is a favourite now. The key must name something
// that exists, so a stale page cannot grow the file with ghosts.
func (a *App) ToggleFavorite(key string) (bool, error) {
	if !usage.ValidKey(key) || !a.exists(key) {
		return false, fmt.Errorf("%w: %s", errUnknownItem, key)
	}

	on, err := a.usage.ToggleFavorite(key)
	if err != nil {
		log.Printf("favorite %s: %v", key, err)
	}

	return on, err
}

// recordOpen counts an opening; a failure to save is logged, never shown,
// because the app or link did open.
func (a *App) recordOpen(key string) {
	if err := a.usage.RecordOpen(key); err != nil {
		log.Printf("usage: %v", err)
	}
}

// findLink looks a link up by id in the current library.
func (a *App) findLink(id string) (library.Link, bool) {
	for _, link := range a.library.Snapshot().Links {
		if link.ID == id {
			return link, true
		}
	}

	return library.Link{}, false
}

// findDiscovered looks a discovered app up, running the discovery when the
// page has not asked for the list yet in this run.
func (a *App) findDiscovered(id string) (discover.App, bool) {
	a.mu.Lock()
	found, ok := a.discovered[id]
	a.mu.Unlock()

	if ok {
		return found, true
	}

	for _, app := range a.discover(a.library.Snapshot().Settings) {
		if app.ID == id {
			return app, true
		}
	}

	return discover.App{}, false
}

// appTarget is the argument list `open` needs for an app id.
func (a *App) appTarget(id string) ([]string, error) {
	for _, app := range a.library.Snapshot().Apps {
		if app.ID != id {
			continue
		}

		if app.BundleID != "" {
			return []string{"-b", app.BundleID}, nil
		}

		if _, err := os.Stat(app.Path); err != nil {
			return nil, fmt.Errorf("%w: %s", errNotInstalled, app.Path)
		}

		return []string{app.Path}, nil
	}

	if found, ok := a.findDiscovered(id); ok {
		return []string{found.Path}, nil
	}

	return nil, fmt.Errorf("%w: %s", errUnknownItem, id)
}

// appPath is the bundle path of an app id, for the Finder.
func (a *App) appPath(id string) (string, error) {
	for _, app := range a.library.Snapshot().Apps {
		if app.ID == id {
			if app.Path == "" {
				return "", errNoPath
			}

			return app.Path, nil
		}
	}

	if found, ok := a.findDiscovered(id); ok {
		return found.Path, nil
	}

	return "", fmt.Errorf("%w: %s", errUnknownItem, id)
}

// exists reports whether a usage key names a current item.
func (a *App) exists(key string) bool {
	tab, id, _ := strings.Cut(key, ":")

	if tab == tabLinks {
		_, ok := a.findLink(id)
		return ok
	}

	for _, app := range a.library.Snapshot().Apps {
		if app.ID == id {
			return true
		}
	}

	_, ok := a.findDiscovered(id)

	return ok
}
