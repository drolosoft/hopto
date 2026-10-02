//go:build windows

package discover

// Inspect reads one app the way this platform installs them: a shortcut.
func Inspect(path string) (App, error) {
	return InspectShortcut(path)
}
