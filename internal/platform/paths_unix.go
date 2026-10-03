//go:build !windows

package platform

import "path/filepath"

// DataDir is where hopto keeps the user's files: the library, the usage
// counts and the icons, private to the user.
func DataDir(home string) string {
	return filepath.Join(home, "Library", "Application Support", "hopto")
}

// logPath is ~/Library/Logs/hopto.log: an app opened with `open` has no
// terminal, so the file is the only place to see what happened.
func logPath(home string) string {
	return filepath.Join(home, "Library", "Logs", "hopto.log")
}
