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

// errPanelHidden answers PickApp when a hotkey hid the panel just before
// the sheet could attach: a sheet on a hidden window never gets an
// answer, which would leave dialogOpen stuck refusing toggle, Hide and
// reopen until Quit.
var errPanelHidden = errors.New("the panel is hidden")

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
	if err := a.beginDialog(); err != nil {
		return AppDraft{}, err
	}
	defer a.endDialog()

	path, err := a.window.PickFile(pickFolder())
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
// floating level. It refuses a second dialog while one is already open,
// and refuses one on a hidden panel: a hotkey hide landing between the
// page asking for the dialog and the sheet attaching would otherwise
// leave a sheet nobody can answer, and dialogOpen stuck until Quit.
func (a *App) beginDialog() error {
	a.mu.Lock()
	if a.dialogOpen {
		a.mu.Unlock()
		return errDialogBusy
	}

	if !a.visible {
		a.mu.Unlock()
		return errPanelHidden
	}

	a.dialogOpen = true
	a.mu.Unlock()

	a.window.SetAlwaysOnTop(false)

	return nil
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

// appDuplicate finds the entry that already opens this app: one added
// by hand first (the editor offers to edit it), then a discovered one
// (it is already on the grid, and saving adopts it).
func (a *App) appDuplicate(path, bundleID string) *Ref {
	lib := a.library.Snapshot()
	if ref := handAddedDuplicate(lib, path, bundleID); ref != nil {
		return ref
	}

	for _, found := range a.discover(lib.Settings) {
		if sameApp(path, bundleID, found.Path, found.BundleID) {
			return &Ref{Tab: tabApps, ID: found.ID, Name: found.Name}
		}
	}

	return nil
}

// sameApp reports whether two entries open the same app: the same
// bundle path, or the same bundle id. An empty path or bundle id
// matches nothing, so two entries that only have the other field never
// look alike by accident.
func sameApp(path, bundleID, otherPath, otherBundle string) bool {
	samePath := path != "" && otherPath == path
	sameBundle := bundleID != "" && otherBundle == bundleID

	return samePath || sameBundle
}

// handAddedDuplicate is the entry of lib added by hand that already
// opens the app at path (or with bundleID), or nil. The picked draft
// and AddApp share it, so the page and Go can never disagree on what
// counts as a twin.
func handAddedDuplicate(lib library.Library, path, bundleID string) *Ref {
	for _, app := range lib.Apps {
		if sameApp(path, bundleID, app.Path, app.BundleID) {
			return &Ref{Tab: tabApps, ID: app.ID, Name: app.Name}
		}
	}

	return nil
}
