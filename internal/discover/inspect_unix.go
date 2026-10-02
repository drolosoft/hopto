//go:build !windows

package discover

// Inspect reads one app the way this platform installs them: a bundle.
func Inspect(path string) (App, error) {
	return InspectBundle(path)
}
