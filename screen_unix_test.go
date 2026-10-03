//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

// window.json is private to the user. A POSIX mode says so; on Windows a
// mode is a single read-only bit, so this check runs everywhere else.
func TestWindowStateIsPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hopto", windowFile)

	if err := writeWindowState(path, 69733378); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode %o, want 600", info.Mode().Perm())
	}
}
