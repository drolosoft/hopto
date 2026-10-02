package main

import (
	"log"
	"os"
	"strings"
	"testing"
)

// A brand-new home has no logs folder yet; openLog must create
// it rather than give up silently, or the app leaves no trace to read.
func TestOpenLogCreatesTheFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	openLog()
	log.Printf("hello from the test")

	path := logPath(home)

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}

	if !strings.Contains(string(written), "hello from the test") {
		t.Errorf("log file does not have the line: %q", written)
	}
}
