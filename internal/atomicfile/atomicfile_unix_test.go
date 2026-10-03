//go:build !windows

package atomicfile

import (
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
)

// These tests rely on POSIX promises: modes that keep a file private,
// and a rename that replaces the destination atomically while other
// writers race on it. Windows makes neither (a mode there is one
// read-only bit, and MoveFileEx may fail with access denied while
// another replace of the same file is under way), so they run everywhere
// but Windows.

// Twenty writers racing on the same path end with a whole file that is one
// of the versions written, never a mix or an empty file (the -race flag
// covers the memory side).
func TestConcurrentWritesLeaveAWholeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	var group sync.WaitGroup

	// One version per writer, recorded up front so the final file can be
	// checked against the exact bytes a writer sent, not just their length.
	versions := make([]string, 20)
	for writer := range 20 {
		versions[writer] = string(rune('a'+writer)) + "-version-of-the-file"
	}

	for writer := range 20 {
		group.Go(func() {
			if err := Write(path, []byte(versions[writer]), 0o600); err != nil {
				t.Error(err)
			}
		})
	}

	group.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if !slices.Contains(versions, string(data)) {
		t.Fatalf("file is %q, not one of the twenty written versions", data)
	}

	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
}

// The file is private to the user, and so is a folder the write had to
// create: together they hold the library and the usage counts.
func TestWriteKeepsTheFilesPrivate(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "hopto")
	path := filepath.Join(folder, "usage.json")

	if err := Write(path, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o, want 600", info.Mode().Perm())
	}

	info, err = os.Stat(folder)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o700 {
		t.Fatalf("folder mode %o, want 700", info.Mode().Perm())
	}
}
