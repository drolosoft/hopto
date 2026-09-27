package usage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// A fixed clock so lastOpened can be compared exactly.
var fixedNow = time.Date(2026, 9, 26, 20, 0, 0, 0, time.UTC)

// openAt opens a store on a path inside a temporary folder with the fixed
// clock, failing the test on any error.
func openAt(t *testing.T, path string) *Store {
	t.Helper()

	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}

	store.now = func() time.Time { return fixedNow }

	return store
}

// Counts survive a restart: what one store saved, a second one reads.
func TestStoreStartsEmptyAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	store := openAt(t, path)

	if err := store.RecordOpen("links:mdn"); err != nil {
		t.Fatal(err)
	}

	if err := store.RecordOpen("links:mdn"); err != nil {
		t.Fatal(err)
	}

	again := openAt(t, path)
	snapshot := again.Snapshot()

	if snapshot.Opens["links:mdn"] != 2 {
		t.Fatalf("opens = %d, want 2", snapshot.Opens["links:mdn"])
	}

	if !snapshot.LastOpened["links:mdn"].Equal(fixedNow) {
		t.Fatalf("lastOpened = %v, want %v", snapshot.LastOpened["links:mdn"], fixedNow)
	}
}

// The bug of 2026-09-26: a corrupt file used to be replaced on the first
// opening. Now the error is reported, the store refuses to save and the
// bytes on disk stay exactly as they were.
func TestCorruptFileIsNeverOverwritten(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	corrupt := []byte(`{"opens": {"links:mdn": 41}, "favorites": [`)

	if err := os.WriteFile(path, corrupt, 0o600); err != nil {
		t.Fatal(err)
	}

	store, err := Open(path)
	if err == nil {
		t.Fatal("expected an error for a corrupt file")
	}

	if !store.ReadOnly() {
		t.Fatal("a store on a corrupt file must be read only")
	}

	if err := store.RecordOpen("links:mdn"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("RecordOpen error = %v, want ErrReadOnly", err)
	}

	if _, err := store.ToggleFavorite("links:mdn"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("ToggleFavorite error = %v, want ErrReadOnly", err)
	}

	data, _ := os.ReadFile(path)
	if string(data) != string(corrupt) {
		t.Fatalf("the corrupt file was rewritten: %s", data)
	}
}

// A file with "favorites": null (hand edited, or from an older version)
// must never be written back with the same null: the page treats
// favorites as a list, and json.Marshal turns a nil slice into null.
func TestFavoritesNeverWrittenBackAsNull(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")

	if err := os.WriteFile(path, []byte(`{"opens": {}, "lastOpened": {}, "favorites": null}`), 0o600); err != nil {
		t.Fatal(err)
	}

	store := openAt(t, path)

	if err := store.RecordOpen("apps:cronometro"); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	var onDisk map[string]json.RawMessage
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatal(err)
	}

	if string(onDisk["favorites"]) != "[]" {
		t.Fatalf("favorites on disk = %s, want []", onDisk["favorites"])
	}
}

// A file with null maps (hand edited, or from an older version) loads with
// empty maps instead of panicking on the first write.
func TestNullMapsDoNotPanic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")

	if err := os.WriteFile(path, []byte(`{"opens": null, "lastOpened": null, "favorites": null}`), 0o600); err != nil {
		t.Fatal(err)
	}

	store := openAt(t, path)

	if err := store.RecordOpen("apps:cronometro"); err != nil {
		t.Fatal(err)
	}

	if store.Snapshot().Opens["apps:cronometro"] != 1 {
		t.Fatal("the opening was not counted")
	}
}

// Favourites toggle on and off and report the new state.
func TestToggleFavorite(t *testing.T) {
	store := openAt(t, filepath.Join(t.TempDir(), "usage.json"))

	on, err := store.ToggleFavorite("links:mdn")
	if err != nil || !on {
		t.Fatalf("first toggle: on=%v err=%v", on, err)
	}

	on, err = store.ToggleFavorite("links:mdn")
	if err != nil || on {
		t.Fatalf("second toggle: on=%v err=%v", on, err)
	}

	if len(store.Snapshot().Favorites) != 0 {
		t.Fatal("favourite still listed after unmarking")
	}
}

// Forget drops every trace of an item: counts, last opening and favourite.
func TestForgetRemovesEveryTrace(t *testing.T) {
	store := openAt(t, filepath.Join(t.TempDir(), "usage.json"))

	if err := store.RecordOpen("links:mdn"); err != nil {
		t.Fatal(err)
	}

	if _, err := store.ToggleFavorite("links:mdn"); err != nil {
		t.Fatal(err)
	}

	if err := store.Forget("links:mdn"); err != nil {
		t.Fatal(err)
	}

	snapshot := store.Snapshot()
	_, opens := snapshot.Opens["links:mdn"]
	_, last := snapshot.LastOpened["links:mdn"]

	if opens || last || len(snapshot.Favorites) != 0 {
		t.Fatalf("traces left: %+v", snapshot)
	}
}

// Snapshot is a copy and serialises empty collections as [] and {}, never
// null: the page treats them as a list and a map.
func TestSnapshotIsACopyWithoutNulls(t *testing.T) {
	store := openAt(t, filepath.Join(t.TempDir(), "usage.json"))
	snapshot := store.Snapshot()

	snapshot.Favorites = append(snapshot.Favorites, "links:mdn")
	snapshot.Opens["links:mdn"] = 9

	if len(store.Snapshot().Favorites) != 0 || store.Snapshot().Opens["links:mdn"] != 0 {
		t.Fatal("the snapshot shares memory with the store")
	}

	data, _ := json.Marshal(store.Snapshot())
	if string(data) != `{"opens":{},"lastOpened":{},"favorites":[]}` {
		t.Fatalf("json = %s", data)
	}
}

// Keys that the page could forge are refused before touching the file.
func TestInvalidKeysAreRefused(t *testing.T) {
	store := openAt(t, filepath.Join(t.TempDir(), "usage.json"))

	for _, key := range []string{"", "mdn", "links:", "links:MDN", "links:../x", "other:mdn", "links:" + string(make([]byte, 70))} {
		if err := store.RecordOpen(key); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("RecordOpen(%q) error = %v, want ErrInvalidKey", key, err)
		}
	}
}

// Twenty goroutines counting at once never lose an opening (-race covers
// the memory side, the count covers the logic).
func TestConcurrentRecordOpen(t *testing.T) {
	store := openAt(t, filepath.Join(t.TempDir(), "usage.json"))
	var group sync.WaitGroup

	for range 20 {
		group.Go(func() {
			// The key is valid and constant, so this never fails; a
			// goroutine cannot call t.Fatal, hence the plain discard.
			_ = store.RecordOpen("links:mdn")
		})
	}

	group.Wait()

	if store.Snapshot().Opens["links:mdn"] != 20 {
		t.Fatalf("opens = %d, want 20", store.Snapshot().Opens["links:mdn"])
	}
}
