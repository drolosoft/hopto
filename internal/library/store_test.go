package library

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// A tiny valid file used as seed and as fixture.
const sampleTOML = `version = 1

[settings]
language = "auto"
hotkey_apps = "cmd+shift+space"
hotkey_links = "cmd+option+space"
scan_applications = true
discover_edge_apps = true
icon_services = ["site"]

[[categories]]
id = "dev"
name = "Dev"
tab = "links"

[[links]]
id = "github"
name = "GitHub"
url = "https://github.com"
category = "dev"
`

const home = "/Users/someone"

// openSample opens a store on a fresh path with the sample as seed.
func openSample(t *testing.T) (*Store, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "library.toml")
	store, err := Open(path, []byte(sampleTOML), home)
	if err != nil {
		t.Fatal(err)
	}

	return store, path
}

// The first run writes the seed once; a second Open reads it back and does
// not write again.
func TestOpenSeedsAMissingFile(t *testing.T) {
	store, path := openSample(t)

	if len(store.Snapshot().Links) != 1 {
		t.Fatal("seed not loaded")
	}

	first, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := Open(path, []byte(sampleTOML), home); err != nil {
		t.Fatal(err)
	}

	second, _ := os.Stat(path)
	if !second.ModTime().Equal(first.ModTime()) {
		t.Fatal("the file was rewritten on the second open")
	}
}

// An empty file is an empty library, not an error, and is left alone.
func TestOpenAcceptsAnEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.toml")
	os.WriteFile(path, nil, 0o600)

	store, err := Open(path, []byte(sampleTOML), home)
	if err != nil {
		t.Fatal(err)
	}

	if store.Snapshot().Version != CurrentVersion || len(store.Snapshot().Links) != 0 {
		t.Fatalf("snapshot = %+v", store.Snapshot())
	}

	if info, _ := os.Stat(path); info.Size() != 0 {
		t.Fatal("the empty file was rewritten")
	}
}

// A file that does not parse is never rewritten: the store reports the
// line, keeps the seed in memory and refuses every write.
func TestOpenKeepsACorruptFileIntact(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.toml")
	corrupt := []byte("version = 1\n[[links]\nid = \"x\"\n")
	os.WriteFile(path, corrupt, 0o600)

	store, err := Open(path, []byte(sampleTOML), home)
	if err == nil {
		t.Fatal("expected a parse error")
	}

	status := store.Status()
	if !status.ReadOnly || status.Line != 3 {
		t.Fatalf("status = %+v, want read only at line 3", status)
	}

	applyErr := store.Apply(func(lib *Library) error { lib.Links = nil; return nil })
	if !errors.Is(applyErr, ErrReadOnly) {
		t.Fatalf("Apply error = %v, want ErrReadOnly", applyErr)
	}

	data, _ := os.ReadFile(path)
	if string(data) != string(corrupt) {
		t.Fatalf("file rewritten: %s", data)
	}
}

// A file from the future is refused and left intact.
func TestOpenRefusesAFutureVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.toml")
	os.WriteFile(path, []byte("version = 99\n"), 0o600)

	store, err := Open(path, []byte(sampleTOML), home)
	if !errors.Is(err, ErrFutureVersion) || !store.Status().ReadOnly {
		t.Fatalf("err = %v, status = %+v", err, store.Status())
	}
}

// Apply validates the whole result before writing: a bad operation leaves
// the file and the memory as they were.
func TestApplyRejectsAnInvalidResult(t *testing.T) {
	store, path := openSample(t)
	before, _ := os.ReadFile(path)

	err := store.Apply(func(lib *Library) error {
		lib.Links = append(lib.Links, Link{ID: "bad", Name: "Bad", URL: "file:///x", Category: "dev"})
		return nil
	})

	var problem *Problem
	if !errors.As(err, &problem) || problem.Field != "url" {
		t.Fatalf("err = %v", err)
	}

	after, _ := os.ReadFile(path)
	if string(after) != string(before) || len(store.Snapshot().Links) != 1 {
		t.Fatal("a rejected operation changed the file or the memory")
	}
}

// A good operation reaches the disk, keeps the previous version in .bak,
// and comes back through a fresh Open with the same order.
func TestApplyWritesAndKeepsABackup(t *testing.T) {
	store, path := openSample(t)

	err := store.Apply(func(lib *Library) error {
		lib.Links = append(lib.Links, Link{ID: "mdn", Name: "MDN", URL: "https://developer.mozilla.org", Category: "dev", Keywords: []string{"web"}})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path + ".bak"); err != nil {
		t.Fatal("no .bak after a write")
	}

	again, err := Open(path, []byte(sampleTOML), home)
	if err != nil {
		t.Fatal(err)
	}

	links := again.Snapshot().Links
	if len(links) != 2 || links[0].ID != "github" || links[1].ID != "mdn" || links[1].Keywords[0] != "web" {
		t.Fatalf("links = %+v", links)
	}
}

// A hand edit made while the app is open is picked up before the next
// operation, so the operation adds to the edited file instead of undoing
// the edit.
func TestApplySeesAHandEdit(t *testing.T) {
	store, path := openSample(t)

	edited := sampleTOML + "\n[[links]]\nid = \"by-hand\"\nname = \"By hand\"\nurl = \"https://example.com\"\ncategory = \"dev\"\n"
	os.WriteFile(path, []byte(edited), 0o600)
	// Same second as the seed write is possible on a fast disk: force a
	// different size so the change is noticed either way.

	err := store.Apply(func(lib *Library) error {
		lib.Links = append(lib.Links, Link{ID: "mdn", Name: "MDN", URL: "https://developer.mozilla.org", Category: "dev"})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	ids := []string{}
	for _, link := range store.Snapshot().Links {
		ids = append(ids, link.ID)
	}

	if len(ids) != 3 || ids[1] != "by-hand" || ids[2] != "mdn" {
		t.Fatalf("ids = %v", ids)
	}
}

// A hand edit that breaks the file makes the store read only from the
// next reload, keeping the last good library in memory.
func TestReloadKeepsTheLastGoodLibrary(t *testing.T) {
	store, path := openSample(t)
	os.WriteFile(path, []byte("version = 1\n[[links]\n"), 0o600)

	if err := store.ReloadIfChanged(); err == nil {
		t.Fatal("expected a parse error")
	}

	if len(store.Snapshot().Links) != 1 || !store.Status().ReadOnly {
		t.Fatalf("snapshot lost or store writable: %+v", store.Status())
	}

	os.WriteFile(path, []byte(sampleTOML), 0o600)
	if err := store.ReloadIfChanged(); err != nil || store.Status().ReadOnly {
		t.Fatalf("store did not recover: err=%v status=%+v", err, store.Status())
	}
}

// Snapshot is a deep copy: editing it never touches the store.
func TestSnapshotIsADeepCopy(t *testing.T) {
	store, _ := openSample(t)
	snapshot := store.Snapshot()
	snapshot.Links[0].Name = "changed"
	snapshot.Categories = append(snapshot.Categories, Category{ID: "x", Name: "X", Tab: TabApps})

	if store.Snapshot().Links[0].Name != "GitHub" || len(store.Snapshot().Categories) != 1 {
		t.Fatal("the snapshot shares memory with the store")
	}
}

// Decode fills the defaults an old or hand-written file may lack, and
// reports keys hopto does not know instead of failing on them.
func TestDecodeDefaultsAndUnknownKeys(t *testing.T) {
	lib, err := Decode([]byte("[[categories]]\nid = \"dev\"\nname = \"Dev\"\ntab = \"links\"\ncolour = \"red\"\n"))
	if err != nil {
		t.Fatal(err)
	}

	if lib.Version != CurrentVersion || lib.Links == nil || lib.Settings.Language != "auto" {
		t.Fatalf("lib = %+v", lib)
	}

	unknown := UnknownKeys([]byte("[[categories]]\nid = \"dev\"\nname = \"Dev\"\ntab = \"links\"\ncolour = \"red\"\n"))
	if len(unknown) != 1 || unknown[0] != "categories.colour" {
		t.Fatalf("unknown = %v", unknown)
	}
}

// Encode writes a file Decode reads back unchanged, with the header comment
// on top so a person opening it knows what it is.
func TestEncodeRoundTrip(t *testing.T) {
	lib, _ := Decode([]byte(sampleTOML))
	data, err := Encode(lib)
	if err != nil {
		t.Fatal(err)
	}

	if data[0] != '#' {
		t.Fatalf("no header comment: %s", data[:40])
	}

	back, err := Decode(data)
	if err != nil {
		t.Fatal(err)
	}

	if back.Links[0].URL != "https://github.com" || back.Categories[0].Tab != TabLinks || back.Settings.HotkeyApps != "cmd+shift+space" {
		t.Fatalf("round trip lost data: %+v", back)
	}
}
