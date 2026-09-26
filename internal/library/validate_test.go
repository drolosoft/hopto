package library

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

// A small valid library every case below starts from.
func validLibrary() Library {
	lib := Default()
	lib.Categories = []Category{{ID: "dev", Tab: TabLinks, Name: "Dev"}, {ID: "tools", Tab: TabApps, Name: "Tools"}}
	lib.Links = []Link{{ID: "github", Name: "GitHub", URL: "https://github.com", Category: "dev"}}
	lib.Apps = []AppEntry{{ID: "safari", Name: "Safari", Path: "/Applications/Safari.app", Category: "tools"}}

	return lib
}

func TestValidateAcceptsAValidLibrary(t *testing.T) {
	if err := Validate(validLibrary(), "/Users/someone"); err != nil {
		t.Fatal(err)
	}
}

// Every mistake a hand edit can make, and the field the problem points at.
func TestValidateRejects(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*Library)
		field string
	}{
		{"link id with uppercase", func(l *Library) { l.Links[0].ID = "GitHub" }, "id"},
		{"link id with a colon", func(l *Library) { l.Links[0].ID = "git:hub" }, "id"},
		{"link id reserved for edge apps", func(l *Library) { l.Links[0].ID = "edge-x" }, "id"},
		{"duplicate link id", func(l *Library) { l.Links = append(l.Links, l.Links[0]) }, "id"},
		{"empty name", func(l *Library) { l.Links[0].Name = "" }, "name"},
		{"name too long", func(l *Library) { l.Links[0].Name = strings.Repeat("a", 81) }, "name"},
		{"control char in name", func(l *Library) { l.Links[0].Name = "Git\x00Hub" }, "name"},
		{"description too long", func(l *Library) { l.Links[0].Description = strings.Repeat("a", 201) }, "description"},
		{"unknown category", func(l *Library) { l.Links[0].Category = "nope" }, "category"},
		{"category of the other tab", func(l *Library) { l.Links[0].Category = "tools" }, "category"},
		{"file url", func(l *Library) { l.Links[0].URL = "file:///etc/passwd" }, "url"},
		{"javascript url", func(l *Library) { l.Links[0].URL = "javascript:alert(1)" }, "url"},
		{"custom scheme", func(l *Library) { l.Links[0].URL = "x-apple.systempreferences:x" }, "url"},
		{"url with userinfo", func(l *Library) { l.Links[0].URL = "https://google.com@evil.example/" }, "url"},
		{"url without host", func(l *Library) { l.Links[0].URL = "https:///path" }, "url"},
		{"url with bad port", func(l *Library) { l.Links[0].URL = "https://example.com:99999/" }, "url"},
		{"url with leading dash", func(l *Library) { l.Links[0].URL = "-a Calculator" }, "url"},
		{"url with a zero-width char", func(l *Library) { l.Links[0].URL = "https://exam​ple.com" }, "url"},
		{"url too long", func(l *Library) { l.Links[0].URL = "https://example.com/" + strings.Repeat("a", 2048) }, "url"},
		{"app without path or bundle id", func(l *Library) { l.Apps[0].Path = "" }, "path"},
		{"app path not a bundle", func(l *Library) { l.Apps[0].Path = "/Applications/run.command" }, "path"},
		{"app path relative", func(l *Library) { l.Apps[0].Path = "Applications/Safari.app" }, "path"},
		{"app path with dot dot", func(l *Library) { l.Apps[0].Path = "/Applications/../tmp/x.app" }, "path"},
		{"app path outside the roots", func(l *Library) { l.Apps[0].Path = "/Users/someone/Downloads/x.app" }, "path"},
		{"bad bundle id", func(l *Library) { l.Apps[0].Path = ""; l.Apps[0].BundleID = "not a bundle id" }, "bundle_id"},
		{"category without tab", func(l *Library) { l.Categories[0].Tab = "" }, "tab"},
		{"category id reserved", func(l *Library) { l.Categories[0].ID = "favorites"; l.Links[0].Category = "favorites" }, "id"},
		{"duplicate category", func(l *Library) { l.Categories = append(l.Categories, l.Categories[0]) }, "id"},
		{"hidden with a bad id", func(l *Library) { l.Hidden = []Hidden{{ID: "Bad Id"}} }, "id"},
		{"unknown language", func(l *Library) { l.Settings.Language = "fr" }, "language"},
		{"unknown icon service", func(l *Library) { l.Settings.IconServices = []string{"bing"} }, "icon_services"},
	}

	for _, tc := range cases {
		lib := validLibrary()
		tc.edit(&lib)

		err := Validate(lib, "/Users/someone")

		var problem *Problem
		if !errors.As(err, &problem) {
			t.Errorf("%s: error = %v, want a *Problem", tc.name, err)
			continue
		}

		if problem.Field != tc.field {
			t.Errorf("%s: field = %q, want %q (%v)", tc.name, problem.Field, tc.field, problem)
		}
	}
}

// http is allowed (ttyd, the tailnet) and so is the user's own Applications
// folder; the app is valid whether or not the .app exists on this Mac.
func TestValidateAllows(t *testing.T) {
	lib := validLibrary()
	lib.Links[0].URL = "http://localhost:7681/"
	lib.Apps[0].Path = "/Users/someone/Applications/Edge Apps.localized/Karakeep.app"
	lib.Apps = append(lib.Apps, AppEntry{ID: "by-bundle", Name: "By bundle", BundleID: "com.apple.Safari", Category: "tools"})

	if err := Validate(lib, "/Users/someone"); err != nil {
		t.Fatal(err)
	}
}

// A file from a newer hopto is refused as a whole, so it is never rewritten
// by an older build that would drop the keys it does not know.
func TestValidateRefusesAFutureVersion(t *testing.T) {
	lib := validLibrary()
	lib.Version = CurrentVersion + 1

	if err := Validate(lib, "/Users/someone"); !errors.Is(err, ErrFutureVersion) {
		t.Fatalf("error = %v, want ErrFutureVersion", err)
	}
}

// The size limits keep a runaway file from freezing the page.
func TestValidateRefusesTooManyItems(t *testing.T) {
	lib := validLibrary()
	for index := range maxItems {
		lib.Links = append(lib.Links, Link{ID: "l" + strconv.Itoa(index), Name: "x", URL: "https://example.com", Category: "dev"})
	}

	if err := Validate(lib, "/Users/someone"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("error = %v, want ErrTooLarge", err)
	}
}
