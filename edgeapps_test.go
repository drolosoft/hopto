package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/drolosoft/hopto/internal/usage"
)

// fakeEdgeBundle builds one web app bundle in dir from the test data.
func fakeEdgeBundle(t *testing.T, dir, name string) string {
	t.Helper()

	bundle := filepath.Join(dir, name+".app")
	if err := os.MkdirAll(filepath.Join(bundle, "Contents", "Resources"), 0o755); err != nil {
		t.Fatal(err)
	}

	for src, dst := range map[string]string{
		"testdata/edge-info.plist":              filepath.Join(bundle, "Contents", "Info.plist"),
		"internal/icons/testdata/edge-app.icns": filepath.Join(bundle, "Contents", "Resources", "app.icns"),
	} {
		data, err := os.ReadFile(src)
		if err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(dst, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return bundle
}

// TestScanEdgeAppsBuildsEntries reads a fake Edge apps folder and checks
// name, id, category, URL and the embedded icon.
func TestScanEdgeAppsBuildsEntries(t *testing.T) {
	dir := t.TempDir()
	fakeEdgeBundle(t, dir, "Outlook (PWA)")

	apps := scanEdgeApps(dir)
	if len(apps) != 1 {
		t.Fatalf("got %d apps, want 1", len(apps))
	}

	app := apps[0]
	if app.ID != "edge-outlook-pwa" || app.Name != "Outlook (PWA)" || app.Category != edgeCategory {
		t.Errorf("entry = %+v", app)
	}

	if app.Description != "example.com" {
		t.Errorf("description = %q, want the host of the plist URL", app.Description)
	}

	if !strings.HasPrefix(app.Icon, "data:image/png;base64,") {
		t.Errorf("icon is not a PNG data URL: %.40q", app.Icon)
	}

	if app.Path != filepath.Join(dir, "Outlook (PWA).app") {
		t.Errorf("path = %q", app.Path)
	}
}

// TestScanEdgeAppsMissingFolder is the Mac without Edge apps.
func TestScanEdgeAppsMissingFolder(t *testing.T) {
	if apps := scanEdgeApps(filepath.Join(t.TempDir(), "nope")); len(apps) != 0 {
		t.Errorf("got %d apps from a missing folder", len(apps))
	}
}

// TestSlug covers spaces, brackets and accents. An accented letter is not
// ASCII, so it falls out as a separator like any other punctuation: the id
// stays inside the shape usage.ValidKey requires, which only allows ASCII
// letters and digits.
func TestSlug(t *testing.T) {
	cases := map[string]string{
		"Outlook (PWA)": "outlook-pwa",
		"Go On The Way": "go-on-the-way",
		"bear-writer":   "bear-writer",
		"Música!":       "m-sica",
		"Añadir notas":  "a-adir-notas",
	}

	for in, want := range cases {
		got := slug(in)
		if got != want {
			t.Errorf("slug(%q) = %q, want %q", in, got, want)
		}

		if id := "apps:edge-" + got; !usage.ValidKey(id) {
			t.Errorf("slug(%q) produced id %q, not a valid usage key", in, id)
		}
	}
}
