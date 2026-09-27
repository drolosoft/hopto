package main

import "path/filepath"

// dataDir is where hopto keeps the user's files: the library, the usage
// counts and the icons, private to the user.
func dataDir(home string) string {
	return filepath.Join(home, "Library", "Application Support", "hopto")
}

// The files inside the data folder.
const (
	libraryFile = "library.toml"
	usageFile   = "usage.json"
	windowFile  = "window.json"
	iconsFolder = "icons"
)

// iconsDir is the folder of icons/<id>.png.
func (a *App) iconsDir() string {
	return filepath.Join(a.dataDir, iconsFolder)
}
