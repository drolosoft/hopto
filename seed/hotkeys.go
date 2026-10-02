package seed

import "regexp"

// The two shortcut lines of the seed, matched whole so nothing else in
// the file can be touched.
var (
	appsLine  = regexp.MustCompile(`(?m)^hotkey_apps = ".*"$`)
	linksLine = regexp.MustCompile(`(?m)^hotkey_links = ".*"$`)
)

// withHotkeys returns the seed with its two shortcut lines replaced:
// the links and the categories are the same on every system, only the
// keys differ.
func withHotkeys(data []byte, apps, links string) []byte {
	out := appsLine.ReplaceAll(data, []byte(`hotkey_apps = "`+apps+`"`))

	return linksLine.ReplaceAll(out, []byte(`hotkey_links = "`+links+`"`))
}
