package main

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A brand-new home has no Library/Logs folder yet; openLog must create
// it rather than give up silently, or the app leaves no trace to read.
func TestOpenLogCreatesTheFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	openLog()
	log.Printf("hello from the test")

	path := filepath.Join(home, "Library", "Logs", "hopto.log")

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	if !strings.Contains(string(written), "hello from the test") {
		t.Errorf("log file does not have the line: %q", written)
	}
}
