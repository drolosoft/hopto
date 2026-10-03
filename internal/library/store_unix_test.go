//go:build !windows

package library

import (
	"os"
	"path/filepath"
	"testing"
)

// These tests rely on POSIX folder modes and on an app under
// /Applications, so they run everywhere but Windows. The rest of the
// store tests stay in store_test.go.

// A seed that cannot be written (the folder is read only) is reported by
// Status, not only by the returned error.
func TestOpenReportsASeedThatCannotBeWritten(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	})

	seedPath := filepath.Join(dir, "library.toml")
	store, err := Open(seedPath, []byte(sampleTOML), home)
	if err == nil {
		t.Fatal("expected a write error")
	}

	links := len(store.Snapshot().Links)
	if !store.Status().ReadOnly || links != 1 {
		t.Fatalf("status = %+v, links = %d", store.Status(), links)
	}
}

// An empty apps list must encode without the top-level key, or a
// hand-added [[apps]] table appended afterwards fails to parse with
// "Key 'apps' was already created".
func TestEncodeThenHandAddedAppsTableStillDecodes(t *testing.T) {
	lib := Default()
	lib.Categories = []Category{{ID: "tools", Name: "Tools", Tab: TabApps}}

	data, err := Encode(lib)
	if err != nil {
		t.Fatal(err)
	}

	handAdded := string(data) +
		"\n[[apps]]\nid = \"safari\"\nname = \"Safari\"\n" +
		"path = \"/Applications/Safari.app\"\ncategory = \"tools\"\n"

	decoded, err := Decode([]byte(handAdded))
	if err != nil {
		t.Fatal(err)
	}

	if err := Validate(decoded, home); err != nil {
		t.Fatal(err)
	}

	if len(decoded.Apps) != 1 || decoded.Apps[0].ID != "safari" {
		t.Fatalf("apps = %+v, want one entry for safari", decoded.Apps)
	}
}
