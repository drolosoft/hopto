//go:build windows

package main

import "path/filepath"

// dataDir is %AppData%\hopto, derived from the profile folder rather
// than from the APPDATA variable so a test home holds everything; a
// redirected APPDATA is not followed (ARCHITECTURE.md lists it).
func dataDir(home string) string {
	return filepath.Join(home, "AppData", "Roaming", "hopto")
}

// logPath is %LocalAppData%\hopto\hopto.log: local, not roaming, like
// any log.
func logPath(home string) string {
	return filepath.Join(home, "AppData", "Local", "hopto", "hopto.log")
}
