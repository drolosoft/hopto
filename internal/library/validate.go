package library

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Problem is one thing wrong with one field of one entry. Key is what the
// page translates ("url.scheme"); Detail is for the log, in English.
type Problem struct {
	Field  string
	ID     string
	Key    string
	Detail string
}

// Error makes a Problem usable as an error and readable in the log.
func (p *Problem) Error() string {
	if p.ID == "" {
		return fmt.Sprintf("%s: %s", p.Field, p.Detail)
	}

	return fmt.Sprintf("%s (%s): %s", p.ID, p.Field, p.Detail)
}

// ErrFutureVersion is returned for a file written by a newer hopto.
var ErrFutureVersion = errors.New("library: the file comes from a newer version of hopto")

// ErrTooLarge is returned when the file holds more than the page can show.
var ErrTooLarge = errors.New("library: too many entries")

// The limits of a library. 2000 items is far beyond what a launcher lists,
// and keeps a runaway file from freezing the page; 64 categories is more
// than fit in the chips row.
const (
	maxItems      = 2000
	maxCategories = 64
	maxNameRunes  = 80
	maxDescRunes  = 200
	maxURLBytes   = 2048
)

// idPattern is the shape of every id: a slug of at most 64 characters. Ids
// name icon files and usage keys, so nothing else is allowed.
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// bundlePattern is a reverse-DNS bundle identifier, as `open -b` wants it.
var bundlePattern = regexp.MustCompile(`^[A-Za-z0-9-]+(\.[A-Za-z0-9-]+)+$`)

// reservedPrefixes are the ids of discovered apps; a user entry must not
// look like one, or hiding and usage keys would get confused.
var reservedPrefixes = []string{"edge-", "app-"}

// reservedCategoryIDs are the virtual chips of the page.
var reservedCategoryIDs = map[string]bool{"favoritos": true, "favorites": true}

// The values settings accept.
var (
	languages    = map[string]bool{"auto": true, "es": true, "en": true}
	iconServices = map[string]bool{"site": true, "duckduckgo": true, "google": true}
)

// Validate checks a whole library: the version, the limits, every entry and
// the references between them. The first problem is returned; hand edits
// are fixed one at a time anyway.
func Validate(lib Library, home string) error {
	if lib.Version > CurrentVersion {
		return ErrFutureVersion
	}

	if len(lib.Links)+len(lib.Apps) > maxItems || len(lib.Categories) > maxCategories {
		return ErrTooLarge
	}

	if err := checkSettings(lib.Settings); err != nil {
		return err
	}

	tabs := make(map[string]string, len(lib.Categories))
	for _, category := range lib.Categories {
		if problems := CheckCategory(category); len(problems) > 0 {
			return &problems[0]
		}

		if _, seen := tabs[category.ID]; seen {
			return &Problem{Field: "id", ID: category.ID, Key: "id.duplicate", Detail: "duplicate category id"}
		}

		tabs[category.ID] = category.Tab
	}

	seen := make(map[string]bool, len(lib.Links))
	for _, link := range lib.Links {
		if problems := CheckLink(link); len(problems) > 0 {
			return &problems[0]
		}

		if seen[link.ID] {
			return &Problem{Field: "id", ID: link.ID, Key: "id.duplicate", Detail: "duplicate link id"}
		}

		seen[link.ID] = true

		if err := checkCategoryRef(link.ID, link.Category, TabLinks, tabs); err != nil {
			return err
		}
	}

	seen = make(map[string]bool, len(lib.Apps))
	for _, app := range lib.Apps {
		if problems := CheckApp(app, home); len(problems) > 0 {
			return &problems[0]
		}

		if seen[app.ID] {
			return &Problem{Field: "id", ID: app.ID, Key: "id.duplicate", Detail: "duplicate app id"}
		}

		seen[app.ID] = true

		if err := checkCategoryRef(app.ID, app.Category, TabApps, tabs); err != nil {
			return err
		}
	}

	for _, hidden := range lib.Hidden {
		if !idPattern.MatchString(hidden.ID) {
			return &Problem{Field: "id", ID: hidden.ID, Key: "id.invalid", Detail: "hidden entry with an invalid id"}
		}
	}

	return nil
}

// CheckID accepts the slug shape and refuses the prefixes of discovered
// apps. It returns nil when the id is fine.
func CheckID(id string) *Problem {
	if !idPattern.MatchString(id) {
		return &Problem{Field: "id", ID: id, Key: "id.invalid", Detail: "id must be lowercase letters, digits and dashes, 1 to 64 characters"}
	}

	for _, prefix := range reservedPrefixes {
		if strings.HasPrefix(id, prefix) {
			return &Problem{Field: "id", ID: id, Key: "id.reserved", Detail: "ids starting with " + prefix + " belong to discovered apps"}
		}
	}

	return nil
}

// CheckLink checks the fields of one link on their own; whether the
// category exists is checked against the whole library by Validate.
func CheckLink(link Link) []Problem {
	var problems []Problem

	if problem := CheckID(link.ID); problem != nil {
		problems = append(problems, *problem)
	}

	problems = append(problems, checkText("name", link.ID, link.Name, maxNameRunes, true)...)
	problems = append(problems, checkText("description", link.ID, link.Description, maxDescRunes, false)...)

	if _, err := HostOf(link.URL); err != nil {
		problems = append(problems, Problem{Field: "url", ID: link.ID, Key: "url.invalid", Detail: err.Error()})
	}

	for _, keyword := range link.Keywords {
		problems = append(problems, checkText("keywords", link.ID, keyword, maxNameRunes, true)...)
	}

	return problems
}

// CheckApp checks one app entry. A path is only accepted when it is an
// absolute, clean ".app" folder under the roots macOS installs apps in;
// anything else (a .command, a script, a bundle in Downloads) could run
// code the user never meant to launch.
func CheckApp(app AppEntry, home string) []Problem {
	var problems []Problem

	if problem := CheckID(app.ID); problem != nil {
		problems = append(problems, *problem)
	}

	problems = append(problems, checkText("name", app.ID, app.Name, maxNameRunes, true)...)
	problems = append(problems, checkText("description", app.ID, app.Description, maxDescRunes, false)...)

	if app.Path == "" && app.BundleID == "" {
		problems = append(problems, Problem{Field: "path", ID: app.ID, Key: "app.target", Detail: "an app needs a path or a bundle_id"})
	}

	if app.BundleID != "" && !bundlePattern.MatchString(app.BundleID) {
		problems = append(problems, Problem{Field: "bundle_id", ID: app.ID, Key: "app.bundle", Detail: "bundle_id must look like com.example.App"})
	}

	if app.Path != "" {
		if detail := checkAppPath(app.Path, home); detail != "" {
			problems = append(problems, Problem{Field: "path", ID: app.ID, Key: "app.path", Detail: detail})
		}
	}

	return problems
}

// CheckCategory checks one chip.
func CheckCategory(category Category) []Problem {
	var problems []Problem

	if !idPattern.MatchString(category.ID) || reservedCategoryIDs[category.ID] {
		problems = append(problems, Problem{Field: "id", ID: category.ID, Key: "id.invalid", Detail: "invalid or reserved category id"})
	}

	problems = append(problems, checkText("name", category.ID, category.Name, maxNameRunes, true)...)

	if category.Tab != TabApps && category.Tab != TabLinks {
		problems = append(problems, Problem{Field: "tab", ID: category.ID, Key: "category.tab", Detail: "tab must be apps or links"})
	}

	return problems
}

// AllowedAppRoots are the folders an app entry may point into.
func AllowedAppRoots(home string) []string {
	return []string{
		"/Applications",
		"/System/Applications",
		filepath.Join(home, "Applications"),
	}
}

// HostOf returns the host of a web URL, or why the string is not one. Only
// http and https are accepted; the URL is handed to `open`, and any other
// scheme could reach another app or the file system.
func HostOf(raw string) (string, error) {
	if len(raw) > maxURLBytes {
		return "", errors.New("URL longer than 2048 bytes")
	}

	if strings.ContainsFunc(raw, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || isInvisible(r) }) {
		return "", errors.New("URL contains spaces, control or invisible characters")
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("scheme %q is not http or https", parsed.Scheme)
	}

	if parsed.Hostname() == "" {
		return "", errors.New("URL has no host")
	}

	if parsed.User != nil {
		return "", errors.New("URL carries a user name")
	}

	if port := parsed.Port(); port != "" {
		number, err := strconv.Atoi(port)
		if err != nil || number < 1 || number > 65535 {
			return "", fmt.Errorf("port %q is out of range", port)
		}
	}

	return parsed.Host, nil
}

// isInvisible catches the zero-width and bidi characters that make a URL
// look like another one.
func isInvisible(r rune) bool {
	switch r {
	case 0x200B, 0x200C, 0x200D, 0xFEFF, 0x2060:
		return true
	}

	return (r >= 0x202A && r <= 0x202E) || (r >= 0x2066 && r <= 0x2069)
}

// checkText applies the length and character rules of names, descriptions
// and keywords.
func checkText(field, id, text string, maxRunes int, required bool) []Problem {
	if text == "" {
		if required {
			return []Problem{{Field: field, ID: id, Key: field + ".required", Detail: field + " is required"}}
		}

		return nil
	}

	if utf8.RuneCountInString(text) > maxRunes {
		return []Problem{{Field: field, ID: id, Key: field + ".long", Detail: fmt.Sprintf("%s longer than %d characters", field, maxRunes)}}
	}

	if strings.ContainsFunc(text, unicode.IsControl) {
		return []Problem{{Field: field, ID: id, Key: field + ".control", Detail: field + " contains control characters"}}
	}

	return nil
}

// checkAppPath returns why a path is not acceptable, or "" when it is.
func checkAppPath(path, home string) string {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return "path must be absolute and clean"
	}

	if filepath.Ext(path) != ".app" {
		return "path must end in .app"
	}

	for _, root := range AllowedAppRoots(home) {
		if strings.HasPrefix(path, root+"/") {
			return ""
		}
	}

	return "path must be under /Applications, /System/Applications or ~/Applications"
}

// checkCategoryRef makes sure an entry points at a chip of its own tab.
func checkCategoryRef(id, category, tab string, tabs map[string]string) error {
	found, ok := tabs[category]
	if !ok {
		return &Problem{Field: "category", ID: id, Key: "category.unknown", Detail: "unknown category " + category}
	}

	if found != tab {
		return &Problem{Field: "category", ID: id, Key: "category.tab", Detail: "category " + category + " belongs to the other tab"}
	}

	return nil
}

// checkSettings accepts only the values the app understands.
func checkSettings(settings Settings) error {
	if !languages[settings.Language] {
		return &Problem{Field: "language", Key: "settings.language", Detail: "language must be auto, es or en"}
	}

	for _, service := range settings.IconServices {
		if !iconServices[service] {
			return &Problem{Field: "icon_services", Key: "settings.icons", Detail: "unknown icon service " + service}
		}
	}

	if settings.HotkeyApps == "" || settings.HotkeyLinks == "" {
		return &Problem{Field: "hotkey_apps", Key: "settings.hotkey", Detail: "hotkeys cannot be empty"}
	}

	return nil
}
