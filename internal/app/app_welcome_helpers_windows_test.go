//go:build windows

package app

import "testing"

// finderOff does nothing on Windows: there is no Finder shortcut to
// turn off, and the platform's FinderConflict is always false there.
func finderOff(t *testing.T, app *App) {
	t.Helper()
}
