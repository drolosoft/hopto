package main

import "path/filepath"

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
