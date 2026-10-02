package library

// platformRules is what differs between systems in what the library
// accepts: where an app may live and how it is named, what a secondary
// browser looks like, and the shortcuts a library starts with. The
// value lives in rules_unix.go and rules_windows.go; Validate and
// CheckApp keep their signatures.
type platformRules struct {
	// appExtensions are the file name endings an app path may have.
	appExtensions []string

	// appRoots lists the folders an app path must be under.
	appRoots func(home string) []string

	// appPathHint words the refusal of a path outside the roots.
	appPathHint string

	// underRoot says whether path sits inside root, the way the file
	// system of the platform compares names.
	underRoot func(path, root string) bool

	// browserOK says whether a secondary_browser value is usable, and
	// browserHint words the refusal.
	browserOK   func(value string) bool
	browserHint string

	// The shortcuts of a library that names none.
	defaultAppsHotkey  string
	defaultLinksHotkey string
}
