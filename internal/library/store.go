package library

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/BurntSushi/toml"

	"github.com/drolosoft/hopto/internal/atomicfile"
)

// ErrReadOnly is returned by Apply while the file on disk cannot be parsed
// or comes from a newer version: hopto never rewrites a file it does not
// understand.
var ErrReadOnly = errors.New("library: the file cannot be parsed, refusing to overwrite it")

// header opens every file hopto writes. The comments a user adds are lost
// on the next write, and the header says so.
const header = `# hopto library. The app rewrites this file when you add, edit or delete;
# you can edit it by hand too: it is read again every time the panel opens.
# Comments you add are lost on the next write.

`

// filePerm keeps the library private: it lists the user's sites and apps.
const filePerm = 0o600

// Status is what the page shows about the file: where it is and, when it
// cannot be used, why and on which line.
type Status struct {
	Path     string
	Error    string
	Line     int
	ReadOnly bool
}

// Store keeps the library in memory and on disk. Operations are deltas on
// the file as it is right now, never "save what I have": a hand edit made
// while the app is open is read before the operation is applied.
type Store struct {
	path string
	home string

	mu       sync.Mutex
	lib      Library
	loadErr  error
	loadLine int
	modTime  time.Time
	size     int64
}

// Open loads the file at path. A missing file is created from seed, once.
// A file that does not parse, or comes from a newer hopto, is left intact:
// the store keeps the seed in memory, reports the error and refuses writes
// until the file is fixed.
func Open(path string, seed []byte, home string) (*Store, error) {
	store := &Store{path: path, home: home}

	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		lib, err := Decode(seed)
		if err != nil {
			return store, err
		}

		if err := Validate(lib, home); err != nil {
			return store, err
		}

		store.lib = lib

		return store, store.write(lib)
	}

	if err != nil {
		store.loadErr = err
		store.lib, _ = Decode(seed)
		return store, err
	}

	store.recordStat()

	if err := store.load(data); err != nil {
		store.lib, _ = Decode(seed)
		return store, err
	}

	return store, nil
}

// Path is where the file lives, for "reveal in Finder".
func (s *Store) Path() string {
	return s.path
}

// Snapshot returns a deep copy the page can keep.
func (s *Store) Snapshot() Library {
	s.mu.Lock()
	defer s.mu.Unlock()

	return clone(s.lib)
}

// Status reports the file state for the page.
func (s *Store) Status() Status {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := Status{Path: s.path, ReadOnly: s.loadErr != nil, Line: s.loadLine}
	if s.loadErr != nil {
		status.Error = s.loadErr.Error()
	}

	return status
}

// ReloadIfChanged re-reads the file when its size or modification time
// changed since the last read. Called on every appearance of the panel; a
// broken edit keeps the last good library and turns the store read only.
func (s *Store) ReloadIfChanged() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.reloadIfChanged()
}

// Apply runs op on a copy of the current library, validates the result,
// writes it and only then replaces the memory. A rejected op changes
// nothing. When the store is read only after the reload, the error
// satisfies errors.Is(err, ErrReadOnly), with the parse detail behind it.
func (s *Store) Apply(op func(*Library) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// A file that does not parse, or comes from a newer hopto, is never
	// rewritten: the caller gets ErrReadOnly with the parse detail behind
	// it, so the page can show the line while refusing the edit.
	if err := s.reloadIfChanged(); err != nil || s.loadErr != nil {
		if s.loadErr != nil {
			return fmt.Errorf("%w: %v", ErrReadOnly, s.loadErr)
		}

		return err
	}

	draft := clone(s.lib)
	if err := op(&draft); err != nil {
		return err
	}

	if err := Validate(draft, s.home); err != nil {
		return err
	}

	if err := s.write(draft); err != nil {
		return err
	}

	s.lib = draft

	return nil
}

// reloadIfChanged is ReloadIfChanged with the lock held.
func (s *Store) reloadIfChanged() error {
	info, err := os.Stat(s.path)
	if err != nil {
		return err
	}

	if info.ModTime().Equal(s.modTime) && info.Size() == s.size {
		return s.loadErr
	}

	data, err := os.ReadFile(s.path)
	if err != nil {
		return err
	}

	s.recordStat()

	return s.load(data)
}

// load parses and validates data into the store, recording the failure so
// Status can show it and Apply can refuse. The previous library stays.
func (s *Store) load(data []byte) error {
	lib, err := Decode(data)
	if err == nil {
		err = Validate(lib, s.home)
	}

	if err != nil {
		s.loadErr = err
		s.loadLine = lineOf(err)
		return err
	}

	s.lib = lib
	s.loadErr = nil
	s.loadLine = 0

	return nil
}

// write encodes and saves, keeping the previous file in .bak, and records
// the new stat so the next reload knows this write was ours.
func (s *Store) write(lib Library) error {
	data, err := Encode(lib)
	if err != nil {
		return err
	}

	if err := atomicfile.WriteWithBackup(s.path, data, filePerm); err != nil {
		return err
	}

	s.recordStat()

	return nil
}

// recordStat remembers the size and time of the file as we last saw it.
func (s *Store) recordStat() {
	info, err := os.Stat(s.path)
	if err != nil {
		return
	}

	s.modTime = info.ModTime()
	s.size = info.Size()
}

// lineOf digs the line number out of a TOML parse error, 0 for anything else.
func lineOf(err error) int {
	var parseErr toml.ParseError
	if errors.As(err, &parseErr) {
		return parseErr.Position.Line
	}

	return 0
}

// Decode parses TOML into a Library and fills what a hand-written file may
// leave out: the version, the default settings and empty lists instead of
// nil ones (nil would reach the page as null).
func Decode(data []byte) (Library, error) {
	lib := Default()

	if _, err := toml.Decode(string(data), &lib); err != nil {
		return Library{}, err
	}

	if lib.Version == 0 {
		lib.Version = CurrentVersion
	}

	if lib.Settings.Language == "" {
		lib.Settings.Language = "auto"
	}

	if lib.Settings.HotkeyApps == "" {
		lib.Settings.HotkeyApps = Default().Settings.HotkeyApps
	}

	if lib.Settings.HotkeyLinks == "" {
		lib.Settings.HotkeyLinks = Default().Settings.HotkeyLinks
	}

	return withoutNils(lib), nil
}

// UnknownKeys lists the keys of data that hopto does not understand, for
// the log; a newer hopto may have written them, and they are kept out of
// the way rather than treated as errors.
func UnknownKeys(data []byte) []string {
	var lib Library

	meta, err := toml.Decode(string(data), &lib)
	if err != nil {
		return []string{}
	}

	keys := []string{}
	for _, key := range meta.Undecoded() {
		keys = append(keys, key.String())
	}

	return keys
}

// Encode writes the library as TOML with the header on top.
func Encode(lib Library) ([]byte, error) {
	var buffer bytes.Buffer
	buffer.WriteString(header)

	if err := toml.NewEncoder(&buffer).Encode(withoutNils(lib)); err != nil {
		return nil, err
	}

	return buffer.Bytes(), nil
}

// clone deep-copies a library, slices included.
func clone(lib Library) Library {
	copied := lib
	copied.Settings.IconServices = append([]string{}, lib.Settings.IconServices...)
	copied.Categories = append([]Category{}, lib.Categories...)
	copied.Apps = append([]AppEntry{}, lib.Apps...)
	copied.Hidden = append([]Hidden{}, lib.Hidden...)
	copied.Links = make([]Link, len(lib.Links))

	for index, link := range lib.Links {
		link.Keywords = append([]string{}, link.Keywords...)
		copied.Links[index] = link
	}

	return copied
}

// withoutNils replaces nil slices with empty ones.
func withoutNils(lib Library) Library {
	return clone(lib)
}
