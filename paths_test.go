package main

import (
	"path/filepath"
	"runtime"
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

// On macOS the folders are the ones the system expects, exactly.
func TestMacPathsAreExact(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("the exact paths below are the macOS ones")
	}

	home := "/Users/someone"

	wantData := "/Users/someone/Library/Application Support/hopto"
	if got := dataDir(home); got != wantData {
		t.Errorf("dataDir = %q, want %q", got, wantData)
	}

	wantLog := "/Users/someone/Library/Logs/hopto.log"
	if got := logPath(home); got != wantLog {
		t.Errorf("logPath = %q, want %q", got, wantLog)
	}
}
