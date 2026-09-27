package main

import (
	"encoding/base64"
	"errors"
	"log"
	"path/filepath"

	"github.com/drolosoft/hopto/internal/discover"
	"github.com/drolosoft/hopto/internal/icons"
	"github.com/drolosoft/hopto/internal/library"
)

// errDialogBusy answers a second PickApp while the first panel is up.
var errDialogBusy = errors.New("a file dialog is already open")

// pickFolder is where the open panel starts: where apps are installed.
const pickFolder = "/Applications"

// AppDraft is what the editor pre-fills after the user picks a .app: the
// bundle's name, id and icon, the problem with its path (outside the app
// roots) and the entry that already lists the same app, if any.
type AppDraft struct {
	Path        string `json:"path"`
	BundleID    string `json:"bundleId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	IconDataURL string `json:"iconDataUrl"`
	Problem     string `json:"problem"`
	Duplicate   *Ref   `json:"duplicate"`
}

// PickApp opens the native panel on /Applications and reads the bundle
// the user picks. A cancel is an empty draft, not an error. While the
// panel is up the window stays shown and leaves the floating level, so
// the panel is never under it; both come back when it closes.
func (a *App) PickApp() (AppDraft, error) {
	if !a.beginDialog() {
		return AppDraft{}, errDialogBusy
	}
	defer a.endDialog()

	path, err := a.window.PickFile(pickFolder)
	if err != nil {
		log.Printf("pick app: %v", err)
		return AppDraft{}, err
	}

	if path == "" {
		return AppDraft{}, nil
	}

	return a.appDraft(path), nil
}

// beginDialog marks a dialog as open and takes the window off the
// floating level; false when one is already open.
func (a *App) beginDialog() bool {
	a.mu.Lock()
	if a.dialogOpen {
		a.mu.Unlock()
		return false
	}
	a.dialogOpen = true
	a.mu.Unlock()

	a.window.SetAlwaysOnTop(false)

	return true
}

// endDialog puts the window back on the floating level and gives it the
// keyboard again: the sheet held it, and without this the next key goes
// nowhere until the user clicks.
func (a *App) endDialog() {
	a.mu.Lock()
	a.dialogOpen = false
	a.mu.Unlock()

	a.window.SetAlwaysOnTop(true)
	a.window.Activate()
}

// appDraft reads a picked bundle. The path is cleaned (the panel may end
// it with a slash), an invalid bundle id is dropped rather than refused
// (the path still opens the app), and a path outside the app roots is
// reported for the editor to show under the field.
func (a *App) appDraft(path string) AppDraft {
	path = filepath.Clean(path)
	bundle, err := discover.InspectBundle(path)
	if err != nil {
		log.Printf("pick app %s: %v", path, err)
	}

	draft := AppDraft{
		Path:        path,
		BundleID:    bundle.BundleID,
		Name:        bundle.Name,
		Description: bundle.Description,
	}

	probe := library.AppEntry{
		ID: "draft", Name: draft.Name, Path: path, BundleID: draft.BundleID,
	}
	problems := problemMap(library.CheckApp(probe, a.home))
	if _, bad := problems["bundle_id"]; bad {
		draft.BundleID = ""
	}

	draft.Problem = problems["path"]
	draft.IconDataURL = bundleIconDataURL(bundle.IconPath)
	draft.Duplicate = a.appDuplicate(path, draft.BundleID)

	return draft
}

// bundleIconDataURL is the bundle's icon as a data URL for the editor's
// preview (the draft has no id yet, so no /user-icons/ URL), or "" when
// the bundle has no readable .icns.
func bundleIconDataURL(iconPath string) string {
	if iconPath == "" {
		return ""
	}

	raw, ok := icons.PNG(iconPath)
	if !ok {
		return ""
	}

	png, err := icons.Normalize(raw, icons.AppSide)
	if err != nil {
		return ""
	}

	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
}

// appDuplicate finds the entry that already opens this app: a hand-added
// one first (the editor offers to edit it), then a discovered one (it is
// already on the grid).
func (a *App) appDuplicate(path, bundleID string) *Ref {
	lib := a.library.Snapshot()
	sameApp := func(otherPath, otherBundle string) bool {
		return otherPath == path || (bundleID != "" && otherBundle == bundleID)
	}

	for _, app := range lib.Apps {
		if sameApp(app.Path, app.BundleID) {
			return &Ref{Tab: tabApps, ID: app.ID, Name: app.Name}
		}
	}

	for _, found := range a.discover(lib.Settings) {
		if sameApp(found.Path, found.BundleID) {
			return &Ref{Tab: tabApps, ID: found.ID, Name: found.Name}
		}
	}

	return nil
}
