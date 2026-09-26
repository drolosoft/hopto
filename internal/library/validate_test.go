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

// The baseline every other case in this file starts from and mutates.
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
		{"link id with uppercase", func(lib *Library) { lib.Links[0].ID = "GitHub" }, "id"},
		{"link id with a colon", func(lib *Library) { lib.Links[0].ID = "git:hub" }, "id"},
		{"link id reserved for edge apps", func(lib *Library) { lib.Links[0].ID = "edge-x" }, "id"},
		{"duplicate link id", func(lib *Library) { lib.Links = append(lib.Links, lib.Links[0]) }, "id"},
		{"empty name", func(lib *Library) { lib.Links[0].Name = "" }, "name"},
		{"name too long", func(lib *Library) { lib.Links[0].Name = strings.Repeat("a", 81) }, "name"},
		{"control char in name", func(lib *Library) { lib.Links[0].Name = "Git\x00Hub" }, "name"},
		{"description too long", func(lib *Library) { lib.Links[0].Description = strings.Repeat("a", 201) }, "description"},
		{"unknown category", func(lib *Library) { lib.Links[0].Category = "nope" }, "category"},
		{"category of the other tab", func(lib *Library) { lib.Links[0].Category = "tools" }, "category"},
		{"file url", func(lib *Library) { lib.Links[0].URL = "file:///etc/passwd" }, "url"},
		{"javascript url", func(lib *Library) { lib.Links[0].URL = "javascript:alert(1)" }, "url"},
		{"custom scheme", func(lib *Library) { lib.Links[0].URL = "x-apple.systempreferences:x" }, "url"},
		{"url with userinfo", func(lib *Library) { lib.Links[0].URL = "https://google.com@evil.example/" }, "url"},
		{"url without host", func(lib *Library) { lib.Links[0].URL = "https:///path" }, "url"},
		{"url with bad port", func(lib *Library) { lib.Links[0].URL = "https://example.com:99999/" }, "url"},
		{"url with leading dash", func(lib *Library) { lib.Links[0].URL = "-a Calculator" }, "url"},
		{"url with a zero-width char", func(lib *Library) { lib.Links[0].URL = "https://exam\u200bple.com" }, "url"},
		{"url with a left-to-right mark", func(lib *Library) { lib.Links[0].URL = "https://exam\u200eple.com" }, "url"},
		{"url with a right-to-left mark", func(lib *Library) { lib.Links[0].URL = "https://exam\u200fple.com" }, "url"},
		{"url too long", func(lib *Library) { lib.Links[0].URL = "https://example.com/" + strings.Repeat("a", 2048) }, "url"},
		{"app without path or bundle id", func(lib *Library) { lib.Apps[0].Path = "" }, "path"},
		{"app path not a bundle", func(lib *Library) { lib.Apps[0].Path = "/Applications/run.command" }, "path"},
		{"app path relative", func(lib *Library) { lib.Apps[0].Path = "Applications/Safari.app" }, "path"},
		{"app path with dot dot", func(lib *Library) { lib.Apps[0].Path = "/Applications/../tmp/x.app" }, "path"},
		{"app path outside the roots", func(lib *Library) { lib.Apps[0].Path = "/Users/someone/Downloads/x.app" }, "path"},
		{"bad bundle id", func(lib *Library) { lib.Apps[0].Path = ""; lib.Apps[0].BundleID = "not a bundle id" }, "bundle_id"},
		{"category without tab", func(lib *Library) { lib.Categories[0].Tab = "" }, "tab"},
		{"category id reserved", func(lib *Library) { lib.Categories[0].ID = "favorites"; lib.Links[0].Category = "favorites" }, "id"},
		{"duplicate category", func(lib *Library) { lib.Categories = append(lib.Categories, lib.Categories[0]) }, "id"},
		{"hidden with a bad id", func(lib *Library) { lib.Hidden = []Hidden{{ID: "Bad Id"}} }, "id"},
		{"hidden without a discovered-app prefix", func(lib *Library) { lib.Hidden = []Hidden{{ID: "safari"}} }, "id"},
		{"duplicate hidden id", func(lib *Library) { lib.Hidden = []Hidden{{ID: "edge-x"}, {ID: "edge-x"}} }, "id"},
		{"empty apps hotkey", func(lib *Library) { lib.Settings.HotkeyApps = "" }, "hotkey_apps"},
		{"empty links hotkey", func(lib *Library) { lib.Settings.HotkeyLinks = "" }, "hotkey_links"},
		{"unknown language", func(lib *Library) { lib.Settings.Language = "fr" }, "language"},
		{"unknown icon service", func(lib *Library) { lib.Settings.IconServices = []string{"bing"} }, "icon_services"},
		{"icon over http instead of https", func(lib *Library) { lib.Links[0].Icon = "http://example.com/icon.png" }, "icon"},
		{"icon with a file scheme", func(lib *Library) { lib.Links[0].Icon = "file:///etc/passwd" }, "icon"},
		{"icon hint with a space in the name", func(lib *Library) { lib.Links[0].Icon = "sh:Bad Name" }, "icon"},
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

// Both icon hint shapes are accepted: a selfh.st icon name, or an https
// URL fetched straight into icons/<id>.png.
func TestValidateAcceptsIconHints(t *testing.T) {
	lib := validLibrary()
	lib.Links[0].Icon = "sh:github-light"
	lib.Links = append(lib.Links, Link{ID: "mdn", Name: "MDN", URL: "https://developer.mozilla.org", Category: "dev", Icon: "https://example.com/icon.png"})

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

// A negative version cannot come from any hopto that ever existed; it can
// only be a hand edit gone wrong, and letting it through would send it
// unchanged into the next rewrite.
func TestValidateRefusesANegativeVersion(t *testing.T) {
	lib := validLibrary()
	lib.Version = -1

	err := Validate(lib, "/Users/someone")

	var problem *Problem
	if !errors.As(err, &problem) || problem.Field != "version" || problem.Key != "version.invalid" {
		t.Fatalf("error = %v, want a version.invalid Problem", err)
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
