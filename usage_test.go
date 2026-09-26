package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestUsageStoreStartsEmptyAndPersists counts openings and favourites and
// reads them back from disk through a second store.
func TestUsageStoreStartsEmptyAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "launcher", "usage.json")

	store, err := newUsageStore(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(store.Snapshot().Opens) != 0 || len(store.Snapshot().Favorites) != 0 {
		t.Fatal("a new store should be empty")
	}

	for range 3 {
		if err := store.RecordOpen("links:mdn"); err != nil {
			t.Fatal(err)
		}
	}

	if on, err := store.ToggleFavorite("apps:cronometro"); err != nil || !on {
		t.Fatalf("first toggle = %v, %v; want true, nil", on, err)
	}

	reloaded, err := newUsageStore(path)
	if err != nil {
		t.Fatal(err)
	}

	usage := reloaded.Snapshot()
	if usage.Opens["links:mdn"] != 3 {
		t.Errorf("opens = %d, want 3", usage.Opens["links:mdn"])
	}

	if len(usage.Favorites) != 1 || usage.Favorites[0] != "apps:cronometro" {
		t.Errorf("favorites = %v, want [apps:cronometro]", usage.Favorites)
	}

	if _, err := os.Stat(path + ".tmp"); err == nil {
		t.Error("the temporary file was left behind")
	}
}

// TestToggleFavoriteTwiceRemovesIt checks the second toggle takes the item
// out again.
func TestToggleFavoriteTwiceRemovesIt(t *testing.T) {
	store, err := newUsageStore(filepath.Join(t.TempDir(), "usage.json"))
	if err != nil {
		t.Fatal(err)
	}

	store.ToggleFavorite("links:mdn")

	if on, _ := store.ToggleFavorite("links:mdn"); on {
		t.Error("second toggle should report false")
	}

	if got := store.Snapshot().Favorites; len(got) != 0 {
		t.Errorf("favorites = %v, want none", got)
	}
}

// TestSnapshotIsACopy makes sure the page cannot change the store's state
// through what it was handed.
func TestSnapshotIsACopy(t *testing.T) {
	store, _ := newUsageStore(filepath.Join(t.TempDir(), "usage.json"))
	store.RecordOpen("links:mdn")

	snapshot := store.Snapshot()
	snapshot.Opens["links:mdn"] = 99

	if store.Snapshot().Opens["links:mdn"] != 1 {
		t.Error("snapshot shares the map with the store")
	}
}

// TestNewUsageStoreRejectsCorruptFile reports a broken file instead of
// overwriting it.
func TestNewUsageStoreRejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	os.WriteFile(path, []byte("{not json"), 0o644)

	if _, err := newUsageStore(path); err == nil {
		t.Error("a corrupt file should be reported")
	}
}

// TestSnapshotSerializesEmptyFavoritesAsList is the bug of 2026-09-26: an
// empty store reached the page as {"favorites": null} and the star handler
// crashed on it.
func TestSnapshotSerializesEmptyFavoritesAsList(t *testing.T) {
	store, _ := newUsageStore(filepath.Join(t.TempDir(), "usage.json"))

	data, err := json.Marshal(store.Snapshot())
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != `{"opens":{},"favorites":[]}` {
		t.Errorf("snapshot JSON = %s", data)
	}
}
