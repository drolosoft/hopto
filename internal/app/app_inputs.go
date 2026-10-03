// What the editor sends for a link or an app, how it is tidied into an
// entry, and how the validator's findings come back as problems by field.

package app

import (
	"path/filepath"
	"strings"

	"github.com/drolosoft/hopto/internal/library"
)

// LinkInput is what the editor sends for a link; the id is derived here.
type LinkInput struct {
	URL         string   `json:"url"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Keywords    []string `json:"keywords"`
	Icon        string   `json:"icon"`
}

// AppInput is what the editor sends for a hand-added app.
type AppInput struct {
	Path        string `json:"path"`
	BundleID    string `json:"bundleId"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
}

// tidyLinkInput trims every field, fills a missing scheme and drops empty
// keywords, so what the store validates is what the user meant.
func tidyLinkInput(in LinkInput) LinkInput {
	in.URL = withScheme(strings.TrimSpace(in.URL))
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.Category = strings.TrimSpace(in.Category)
	in.Icon = strings.TrimSpace(in.Icon)

	keywords := []string{}
	for _, keyword := range in.Keywords {
		if keyword = strings.TrimSpace(keyword); keyword != "" {
			keywords = append(keywords, keyword)
		}
	}
	in.Keywords = keywords

	return in
}

// tidyAppInput trims and cleans the path, so "/Applications/Safari.app/"
// matches what the validator wants.
func tidyAppInput(in AppInput) AppInput {
	in.Path = strings.TrimSpace(in.Path)
	if in.Path != "" {
		in.Path = filepath.Clean(in.Path)
	}

	in.BundleID = strings.TrimSpace(in.BundleID)
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	in.Category = strings.TrimSpace(in.Category)

	return in
}

// withScheme puts https:// in front of a bare host or domain path; a value
// with any scheme is left alone for the validator to judge.
func withScheme(raw string) string {
	if raw == "" || strings.Contains(raw, "://") {
		return raw
	}

	return "https://" + raw
}

// linkFromInput builds the entry the store validates.
func linkFromInput(id string, in LinkInput) library.Link {
	return library.Link{
		ID:          id,
		Name:        in.Name,
		Description: in.Description,
		URL:         in.URL,
		Category:    in.Category,
		Keywords:    in.Keywords,
		Icon:        in.Icon,
	}
}

// appFromInput builds the entry the store validates.
func appFromInput(id string, in AppInput) library.AppEntry {
	return library.AppEntry{
		ID:          id,
		Name:        in.Name,
		Description: in.Description,
		BundleID:    in.BundleID,
		Path:        in.Path,
		Category:    in.Category,
	}
}

// linkProblems maps the validator's findings to fields, adding the check
// the validator does against the whole library: the category exists on
// the links tab.
func linkProblems(lib library.Library, link library.Link) map[string]string {
	problems := problemMap(library.CheckLink(link))

	if !hasCategory(lib, tabLinks, link.Category) {
		problems["category"] = "category.unknown"
	}

	return problems
}

// appProblems is linkProblems for an app.
func (a *App) appProblems(
	lib library.Library, app library.AppEntry,
) map[string]string {
	problems := problemMap(library.CheckApp(app, a.home))

	if !hasCategory(lib, tabApps, app.Category) {
		problems["category"] = "category.unknown"
	}

	return problems
}

// problemMap keeps the first problem of each field.
func problemMap(problems []library.Problem) map[string]string {
	byField := map[string]string{}

	for _, problem := range problems {
		if _, seen := byField[problem.Field]; !seen {
			byField[problem.Field] = problem.Key
		}
	}

	return byField
}
