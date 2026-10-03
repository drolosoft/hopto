package atomicfile

import (
	"os"
	"path/filepath"
	"testing"
)

// Nothing but the file is left in the folder: no temporary file survives
// a successful write. The modes are checked in atomicfile_unix_test.go.
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
}

// A write into a folder that does not exist creates it, so the first save
// of a fresh install needs no setup.
func TestWriteCreatesTheFolder(t *testing.T) {
	folder := filepath.Join(t.TempDir(), "hopto")
	path := filepath.Join(folder, "usage.json")

	if err := Write(path, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
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

// A link made before its target exists (a dotfiles setup on a fresh Mac)
// is followed too: the first write creates the target and keeps the link.
func TestWriteFollowsADanglingSymbolicLink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "library.toml")
	link := filepath.Join(dir, "library.toml")

	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := Write(link, []byte("first"), 0o600); err != nil {
		t.Fatal(err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("the dangling link was replaced by a regular file")
	}

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != "first" {
		t.Fatalf("target holds %q, want first", data)
	}
}
