package atomicfile

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// The file is written with the requested mode and nothing else is left in
// the folder: no temporary file survives a successful write.
func TestWriteLeavesOnlyTheFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "usage.json")

	if err := Write(path, []byte(`{"a":1}`), 0o600); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 || entries[0].Name() != "usage.json" {
		t.Fatalf("folder holds %v, want only usage.json", entries)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o, want 600", info.Mode().Perm())
	}
}

// A write into a folder that does not exist creates it, so the first save
// of a fresh install needs no setup.
func TestWriteCreatesTheFolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hopto", "usage.json")

	if err := Write(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

// Twenty writers racing on the same path end with a whole file that is one
// of the versions written, never a mix or an empty file (the -race flag
// covers the memory side).
func TestConcurrentWritesLeaveAWholeFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "usage.json")
	var group sync.WaitGroup

	for writer := range 20 {
		group.Add(1)

		go func() {
			defer group.Done()

			content := []byte(string(rune('a'+writer)) + "-version-of-the-file")
			if err := Write(path, content, 0o600); err != nil {
				t.Error(err)
			}
		}()
	}

	group.Wait()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(data) != len("a-version-of-the-file") {
		t.Fatalf("file is %q, not one whole version", data)
	}

	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("temporary files left behind: %v", entries)
	}
}

// The previous content ends in the .bak file, and the first write of a
// file that does not exist yet creates no .bak.
func TestWriteWithBackupKeepsThePreviousVersion(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.toml")

	if err := WriteWithBackup(path, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path + ".bak"); !os.IsNotExist(err) {
		t.Fatal("a .bak exists after the first write, expected none")
	}

	if err := WriteWithBackup(path, []byte("second"), 0o600); err != nil {
		t.Fatal(err)
	}

	backup, _ := os.ReadFile(path + ".bak")
	current, _ := os.ReadFile(path)

	if string(backup) != "first" || string(current) != "second" {
		t.Fatalf("backup %q current %q", backup, current)
	}
}

// When the path is a symbolic link (a dotfiles setup), the write lands in
// the link's target and the link itself survives.
func TestWriteFollowsASymbolicLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "library.toml")
	link := filepath.Join(dir, "library.toml")

	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := Write(link, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the symbolic link was replaced by a regular file")
	}

	data, _ := os.ReadFile(target)
	if string(data) != "new" {
		t.Fatalf("target holds %q, want new", data)
	}
}
