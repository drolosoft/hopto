// Package usage remembers what the user does with hopto: how many times
// each item was opened and when, so the most used and the most recent
// sort first, and which items are favourites. It lives in its own file,
// apart from the library, because it changes on every opening and the
// library only changes when the user edits it.
package usage

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"time"

	"github.com/drolosoft/hopto/internal/atomicfile"
)

// Usage is the on-disk and in-memory shape. Keys are "<tab>:<id>"
// ("links:mdn", "apps:cronometro") so an app and a link with the same id
// never collide.
type Usage struct {
	Opens      map[string]int       `json:"opens"`
	LastOpened map[string]time.Time `json:"lastOpened"`
	Favorites  []string             `json:"favorites"`
}

// ErrReadOnly is returned by every write when the file on disk could not
// be parsed: the store refuses to replace a file it does not understand.
var ErrReadOnly = errors.New("usage: file is corrupt, refusing to overwrite it")

// ErrInvalidKey is returned for a key that does not look like "<tab>:<id>";
// the page can send any string, so the check happens here.
var ErrInvalidKey = errors.New("usage: invalid key")

// keyPattern is the shape of a usage key: the tab, a colon and a slug of
// at most 64 characters, the same rule as the ids of the library.
var keyPattern = regexp.MustCompile(`^(apps|links):[a-z0-9][a-z0-9-]{0,63}$`)

// filePerm keeps the counts private to the user: they say which sites and
// apps are opened and when.
const filePerm = 0o600

// Store keeps Usage in memory and on disk. Every change is written straight
// away: hopto can be killed at any time and the file is small.
type Store struct {
	path string

	// now is the clock behind lastOpened; tests replace it.
	now func() time.Time

	mu       sync.Mutex
	usage    Usage
	readOnly bool
}

// DefaultPath is the per-user file under Application Support, like every
// other piece of user data of hopto.
func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, "Library", "Application Support", "hopto", "usage.json")
}

// ValidKey reports whether key has the "<tab>:<id>" shape.
func ValidKey(key string) bool {
	return keyPattern.MatchString(key)
}

// Open loads the file at path, or starts empty when it does not exist yet.
// A file that cannot be read or parsed is reported and the returned store
// is read only, so the user's file is never replaced by an empty one.
func Open(path string) (*Store, error) {
	store := &Store{path: path, now: time.Now, usage: emptyUsage()}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}

	if err != nil {
		store.readOnly = true
		return store, err
	}

	if err := json.Unmarshal(data, &store.usage); err != nil {
		store.readOnly = true
		store.usage = emptyUsage()
		return store, err
	}

	// A hand-edited file may carry null maps; the writes below need real ones.
	if store.usage.Opens == nil {
		store.usage.Opens = map[string]int{}
	}

	if store.usage.LastOpened == nil {
		store.usage.LastOpened = map[string]time.Time{}
	}

	return store, nil
}

// ReadOnly reports whether the store refuses to write because the file on
// disk is corrupt.
func (s *Store) ReadOnly() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.readOnly
}

// Snapshot returns a copy the page can keep without racing the store, with
// no nil collection: a nil slice or map reaches the page as JSON null.
func (s *Store) Snapshot() Usage {
	s.mu.Lock()
	defer s.mu.Unlock()

	favorites := slices.Clone(s.usage.Favorites)
	if favorites == nil {
		favorites = []string{}
	}

	opens := make(map[string]int, len(s.usage.Opens))
	for key, count := range s.usage.Opens {
		opens[key] = count
	}

	lastOpened := make(map[string]time.Time, len(s.usage.LastOpened))
	for key, when := range s.usage.LastOpened {
		lastOpened[key] = when
	}

	return Usage{Opens: opens, LastOpened: lastOpened, Favorites: favorites}
}

// RecordOpen counts one more opening of an item, stamps the time and saves.
func (s *Store) RecordOpen(key string) error {
	if !ValidKey(key) {
		return ErrInvalidKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.readOnly {
		return ErrReadOnly
	}

	s.usage.Opens[key]++
	s.usage.LastOpened[key] = s.now()

	return s.save()
}

// ToggleFavorite adds the item to the favourites, or removes it when it is
// already there, and reports the new state. When the save fails the memory
// is rolled back, so the page and the disk never disagree.
func (s *Store) ToggleFavorite(key string) (bool, error) {
	if !ValidKey(key) {
		return false, ErrInvalidKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.readOnly {
		return false, ErrReadOnly
	}

	before := slices.Clone(s.usage.Favorites)
	index := slices.Index(s.usage.Favorites, key)
	on := index < 0

	if on {
		s.usage.Favorites = append(s.usage.Favorites, key)
	} else {
		s.usage.Favorites = slices.Delete(s.usage.Favorites, index, index+1)
	}

	if err := s.save(); err != nil {
		s.usage.Favorites = before
		return !on, err
	}

	return on, nil
}

// Forget drops every trace of an item; the library calls it when the item
// is deleted, so a later item with the same id starts from zero.
func (s *Store) Forget(key string) error {
	if !ValidKey(key) {
		return ErrInvalidKey
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.readOnly {
		return ErrReadOnly
	}

	delete(s.usage.Opens, key)
	delete(s.usage.LastOpened, key)

	if index := slices.Index(s.usage.Favorites, key); index >= 0 {
		s.usage.Favorites = slices.Delete(s.usage.Favorites, index, index+1)
	}

	return s.save()
}

// save writes the file atomically. Called with the lock held.
func (s *Store) save() error {
	if s.path == "" {
		return errors.New("usage: no path to save to")
	}

	data, err := json.MarshalIndent(s.usage, "", "  ")
	if err != nil {
		return err
	}

	return atomicfile.Write(s.path, data, filePerm)
}

// emptyUsage is the starting point of a fresh install and of a store that
// refused a corrupt file.
func emptyUsage() Usage {
	return Usage{
		Opens:      map[string]int{},
		LastOpened: map[string]time.Time{},
		Favorites:  []string{},
	}
}
