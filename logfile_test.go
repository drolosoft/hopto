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
	t.Cleanup(restoreLogOutput)

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

// restoreLogOutput closes the file openLog opened and sends the log back
// to stderr. Closing matters on Windows, where a file still open cannot
// be deleted and the cleanup of the test's home would fail.
func restoreLogOutput() {
	writer := log.Writer()
	log.SetOutput(os.Stderr)

	if file, ok := writer.(*os.File); ok && file != os.Stderr {
		_ = file.Close()
	}
}
