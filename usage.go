package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

// Usage is what the launcher remembers about the user: how many times each
// item was opened, so the most used ones sort first, and which ones are
// favourites. Keys are "<tab>:<id>" ("links:mdn", "apps:cronometro") so
// an app and a link with the same id never collide.
type Usage struct {
	Opens     map[string]int `json:"opens"`
	Favorites []string       `json:"favorites"`
}

// usageStore keeps Usage in memory and on disk. Every change is written
// straight away: the launcher can be killed at any time and the file is
// small.
type usageStore struct {
	path string

	mu    sync.Mutex
	usage Usage
}

// usagePath is the per-user file, under Application Support like every
// other piece of user data of these apps.
func usagePath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, "Library", "Application Support", "hopto", "usage.json")
}

// newUsageStore loads the file at path, or starts empty when it does not
// exist yet. A corrupt file is reported, not silently replaced: the caller
// decides, and the launcher keeps working without counts.
func newUsageStore(path string) (*usageStore, error) {
	store := &usageStore{path: path, usage: Usage{Opens: map[string]int{}}}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}

	if err != nil {
		return store, err
	}

	if err := json.Unmarshal(data, &store.usage); err != nil {
		return store, err
	}

	if store.usage.Opens == nil {
		store.usage.Opens = map[string]int{}
	}

	return store, nil
}

// Snapshot returns a copy the page can keep without racing the store. The
// favourites are never nil: a nil slice reaches the page as JSON null, and
// the page treats them as a list.
func (s *usageStore) Snapshot() Usage {
	s.mu.Lock()
	defer s.mu.Unlock()

	favorites := slices.Clone(s.usage.Favorites)
	if favorites == nil {
		favorites = []string{}
	}

	return Usage{
		Opens:     mapsClone(s.usage.Opens),
		Favorites: favorites,
	}
}

// RecordOpen counts one more opening of an item and saves.
func (s *usageStore) RecordOpen(key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.usage.Opens[key]++

	return s.save()
}

// ToggleFavorite adds the item to the favourites, or removes it when it is
// already there, and reports the new state.
func (s *usageStore) ToggleFavorite(key string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	index := slices.Index(s.usage.Favorites, key)
	if index >= 0 {
		s.usage.Favorites = slices.Delete(s.usage.Favorites, index, index+1)
		return false, s.save()
	}

	s.usage.Favorites = append(s.usage.Favorites, key)

	return true, s.save()
}

// save writes the file through a temporary name and a rename, so a crash
// half way never leaves a truncated file behind. Called with the lock held.
func (s *usageStore) save() error {
	if s.path == "" {
		return errors.New("usage: no path to save to")
	}

	data, err := json.MarshalIndent(s.usage, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}

	temp := s.path + ".tmp"
	if err := os.WriteFile(temp, data, 0o644); err != nil {
		return err
	}

	return os.Rename(temp, s.path)
}

// mapsClone copies a small map; the standard maps.Clone would do, this just
// keeps the nil case explicit.
func mapsClone(source map[string]int) map[string]int {
	clone := make(map[string]int, len(source))
	for key, value := range source {
		clone[key] = value
	}

	return clone
}
