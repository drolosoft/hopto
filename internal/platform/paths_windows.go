//go:build windows

package platform

import "path/filepath"

// DataDir is %AppData%\hopto, derived from the profile folder rather
// than from the APPDATA variable so a test home holds everything. The
// price is that an APPDATA redirected elsewhere, by a roaming profile or
// folder redirection, is not followed.
func DataDir(home string) string {
	return filepath.Join(home, "AppData", "Roaming", "hopto")
}

// logPath is %LocalAppData%\hopto\hopto.log: local, not roaming, like
// any log.
func logPath(home string) string {
	return filepath.Join(home, "AppData", "Local", "hopto", "hopto.log")
}
