package library

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
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
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}

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
	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}

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
	if err := os.WriteFile(path, []byte("version = 99\n"), 0o600); err != nil {
		t.Fatal(err)
	}

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
	// Same second as the seed write is possible on a fast disk: force a
	// different size so the change is noticed either way.
	if err := os.WriteFile(path, []byte(edited), 0o600); err != nil {
		t.Fatal(err)
	}

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
	if err := os.WriteFile(path, []byte("version = 1\n[[links]\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := store.ReloadIfChanged(); err == nil {
		t.Fatal("expected a parse error")
	}

	if len(store.Snapshot().Links) != 1 || !store.Status().ReadOnly {
		t.Fatalf("snapshot lost or store writable: %+v", store.Status())
	}

	if err := os.WriteFile(path, []byte(sampleTOML), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := store.ReloadIfChanged(); err != nil || store.Status().ReadOnly {
		t.Fatalf("store did not recover: err=%v status=%+v", err, store.Status())
	}
}

// A hand edit with the same length as the file, landing in the same
// second as our own write, is still seen: the store compares bytes.
func TestApplySeesASameSizedEditInTheSameSecond(t *testing.T) {
	store, path := openSample(t)

	current, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	// Same length, different content, same timestamp: "GitHub" becomes
	// "GitHab" and the modification time is put back to what it was.
	edited := []byte(strings.Replace(string(current), "GitHub", "GitHab", 1))
	if len(edited) != len(current) {
		t.Fatal("fixture drift: the edit must keep the length")
	}

	if err := os.WriteFile(path, edited, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Chtimes(path, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}

	err = store.Apply(func(lib *Library) error {
		lib.Links = append(lib.Links, Link{ID: "mdn", Name: "MDN", URL: "https://developer.mozilla.org", Category: "dev"})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	if store.Snapshot().Links[0].Name != "GitHab" {
		t.Fatal("the hand edit was overwritten by the operation")
	}
}

// A file that vanishes between operations makes the store read only,
// with Status saying so, until it is back.
func TestApplyRefusesWhenTheFileVanished(t *testing.T) {
	store, path := openSample(t)

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	err := store.Apply(func(lib *Library) error { return nil })
	if !errors.Is(err, ErrReadOnly) || !store.Status().ReadOnly {
		t.Fatalf("err = %v, status = %+v", err, store.Status())
	}

	if err := os.WriteFile(path, []byte(sampleTOML), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := store.Apply(func(lib *Library) error { return nil }); err != nil || store.Status().ReadOnly {
		t.Fatalf("store did not recover: err=%v status=%+v", err, store.Status())
	}
}

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

	store, err := Open(filepath.Join(dir, "library.toml"), []byte(sampleTOML), home)
	if err == nil {
		t.Fatal("expected a write error")
	}

	if !store.Status().ReadOnly || len(store.Snapshot().Links) != 1 {
		t.Fatalf("status = %+v, links = %d", store.Status(), len(store.Snapshot().Links))
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

// Keywords are a slice inside a slice: Snapshot must deep-copy that inner
// slice too, or a page appending to it would silently edit the store.
func TestSnapshotDeepCopiesKeywords(t *testing.T) {
	store, _ := openSample(t)

	err := store.Apply(func(lib *Library) error {
		lib.Links[0].Keywords = []string{"git", "code"}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	snapshot := store.Snapshot()
	snapshot.Links[0].Keywords[0] = "changed"
	snapshot.Links[0].Keywords = append(snapshot.Links[0].Keywords, "extra")

	again := store.Snapshot().Links[0].Keywords
	if again[0] != "git" || len(again) != 2 {
		t.Fatalf("keywords = %v, want the store's own copy untouched", again)
	}
}

// Decode fills the defaults an old or hand-written file may lack, and
// reports keys hopto does not know instead of failing on them.
func TestDecodeDefaultsAndUnknownKeys(t *testing.T) {
	lib, err := Decode([]byte("[[categories]]\nid = \"dev\"\nname = \"Dev\"\ntab = \"links\"\ncolour = \"red\"\n"))
	if err != nil {
		t.Fatal(err)
	}

	wrong := lib.Version != CurrentVersion || lib.Links == nil ||
		lib.Settings.Language != "auto" || lib.Settings.Screen != "last"
	if wrong {
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

// A file past the 1 MiB cap is refused without being read into memory:
// a runaway file (or a symlink loop feeding garbage) must not freeze the
// page trying to parse megabytes of TOML.
func TestOpenRefusesAFileOverTheSizeCap(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.toml")

	// One comment line repeated past the 1 MiB cap; the content does not
	// matter, only its size.
	line := "# padding to cross the one mebibyte cap\n"
	oversized := strings.Repeat(line, maxFileBytes/len(line)+1)
	if err := os.WriteFile(path, []byte(oversized), 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path, []byte(sampleTOML), home)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}

	if !store.Status().ReadOnly {
		t.Fatal("store must be read only over the size cap")
	}
}

// A file that grows past the cap between operations makes the store read
// only from the next reload, the same as a parse error would.
func TestApplyRefusesWhenTheFileGrowsOverTheSizeCap(t *testing.T) {
	store, path := openSample(t)

	line := "# padding to cross the one mebibyte cap\n"
	oversized := strings.Repeat(line, maxFileBytes/len(line)+1)
	if err := os.WriteFile(path, []byte(oversized), 0o600); err != nil {
		t.Fatal(err)
	}

	err := store.Apply(func(lib *Library) error { return nil })
	if !errors.Is(err, ErrReadOnly) {
		t.Fatalf("err = %v, want ErrReadOnly", err)
	}

	if !store.Status().ReadOnly {
		t.Fatal("store must be read only over the size cap")
	}
}

// A file that vanishes and then comes back byte-identical to what the
// store already had in memory must still be read: the checksum recorded
// for the vanished file must not match the recovered one by accident.
func TestApplyRecoversWhenTheFileComesBackIdentical(t *testing.T) {
	store, path := openSample(t)
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	if err := store.Apply(func(lib *Library) error { return nil }); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("err = %v, want ErrReadOnly", err)
	}

	if err := os.WriteFile(path, before, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := store.Apply(func(lib *Library) error { return nil }); err != nil {
		t.Fatal(err)
	}

	if store.Status().ReadOnly {
		t.Fatal("store still read only after the file came back identical")
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

	handAdded := string(data) + "\n[[apps]]\nid = \"safari\"\nname = \"Safari\"\npath = \"/Applications/Safari.app\"\ncategory = \"tools\"\n"

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
