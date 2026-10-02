package main

import (
	"path/filepath"
	"strings"
	"testing"
)

// The data folder is the user's own, under their home, named after the
// app on every system.
func TestDataDirIsUnderHome(t *testing.T) {
	home := t.TempDir()
	dir := dataDir(home)

	if !strings.HasPrefix(dir, home+string(filepath.Separator)) {
		t.Errorf("%s is not under %s", dir, home)
	}

	if filepath.Base(dir) != "hopto" {
		t.Errorf("%s is not named hopto", dir)
	}

	if !strings.HasPrefix(logPath(home), home) {
		t.Errorf("log %s is not under %s", logPath(home), home)
	}
}
