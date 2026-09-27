package main

import (
	"os"
	"path/filepath"

	"github.com/drolosoft/hopto/internal/discover"
)

// Entry is one app of the launcher. Bundle is the .app name on disk; the
// launcher finds it in one of the known locations so the list works both
// from the repository and from an installed copy.
type Entry struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Bundle      string `json:"bundle"`
	Category    string `json:"category"`

	// Path is filled in at runtime with the first existing location, or
	// left empty when the app is not built or installed.
	Path string `json:"path"`

	// Icon is a data URL for apps whose icon is read from the bundle (the
	// Edge web apps); the apps of this repo leave it empty and the page uses
	// the PNG under frontend/src/assets/icons instead.
	Icon string `json:"icon,omitempty"`
}

// appCategories are the filter chips of the apps tab, in screen order. One
// category is enough today; the page treats apps and links the same way, so
// new ones only need a line here and the id on each entry.
var appCategories = []Category{
	{ID: "utilidades", Name: "Utilidades"},
	{ID: discover.SourceEdge, Name: "Apps de Edge"},
}

// catalog lists "mis apps" in the order they appear on screen. Adding an app
// to the catalog means adding a line here and an icon in
// frontend/src/assets/icons.
var catalog = []Entry{
	{ID: "cronometro", Name: "Cronómetro", Description: "Cuenta atrás de 1:05 con semáforo", Bundle: "cronometro.app", Category: "utilidades"},
	{ID: "cunyas", Name: "Cuñas", Description: "Una frase motivacional al azar", Bundle: "cunyas.app", Category: "utilidades"},
}

// locations returns the folders where a bundle may live, most permanent
// first: a system-wide install, a per-user install, and finally the build
// output of a sibling app when the launcher itself runs from the repository.
func locations(repoRoot string, app Entry) []string {
	home, _ := os.UserHomeDir()

	candidates := []string{
		filepath.Join("/Applications", "hopto-apps"),
		filepath.Join(home, "Applications", "hopto-apps"),
		filepath.Join(home, "Applications"),
	}

	if repoRoot != "" {
		candidates = append(candidates, filepath.Join(repoRoot, app.ID, "build", "bin"))
	}

	return candidates
}

// resolve fills Path for every catalog entry and appends the Edge web apps
// found under the home folder, until App is rebuilt on the library.
func resolve(repoRoot string) []Entry {
	apps := resolveCatalog(repoRoot)

	if home, err := os.UserHomeDir(); err == nil {
		edgeDir := discover.EdgeAppsDir(home)
		for _, app := range discover.ScanEdgeApps(edgeDir) {
			apps = append(apps, Entry{
				ID:          app.ID,
				Name:        app.Name,
				Description: app.Description,
				Bundle:      filepath.Base(app.Path),
				Category:    discover.SourceEdge,
				Path:        app.Path,
			})
		}
	}

	return apps
}

// resolveCatalog is resolve for the apps of this repository only.
func resolveCatalog(repoRoot string) []Entry {
	apps := make([]Entry, 0, len(catalog))

	for _, app := range catalog {
		for _, dir := range locations(repoRoot, app) {
			candidate := filepath.Join(dir, app.Bundle)

			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				app.Path = candidate
				break
			}
		}

		apps = append(apps, app)
	}

	return apps
}

// repoRootFromExecutable finds the monorepo checkout from the running
// binary, or returns "" when the launcher runs from somewhere else.
func repoRootFromExecutable() string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}

	return repoRootFrom(executable)
}

// repoRootFrom walks up from a binary path looking for the repository. Inside
// a bundle built with `wails build` the binary sits at
// <repo>/launcher/build/bin/launcher.app/Contents/MacOS/launcher, so the root
// is seven levels up; the go.mod check keeps a stray path from being mistaken
// for it.
func repoRootFrom(executable string) string {
	root := executable
	for range 7 {
		root = filepath.Dir(root)
	}

	if _, err := os.Stat(filepath.Join(root, "launcher", "go.mod")); err != nil {
		return ""
	}

	return root
}
