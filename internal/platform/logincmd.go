package platform

import "strings"

// temporaryMarkers are folder names an exe should never be started from
// at login: a Temp folder is emptied, and a path through a .zip means
// the Explorer is showing the archive, not a real folder.
var temporaryMarkers = []string{`\temp\`, `.zip\`}

// TemporaryExecutable says whether the running exe sits somewhere that
// will not be there at the next login, compared without case.
func TemporaryExecutable(exe string) bool {
	lower := strings.ToLower(exe)
	for _, marker := range temporaryMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}

	return false
}

// QuotedCommand is the Run value: the path in double quotes, since
// Program Files has a space in it.
func QuotedCommand(exe string) string {
	return `"` + exe + `"`
}
