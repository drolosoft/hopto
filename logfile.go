package main

import (
	"log"
	"os"
	"path/filepath"
)

// openLog sends the standard logger to ~/Library/Logs/hopto.log. An app
// opened with `open` has no terminal, so this file is the only place to see
// what happened (for instance whether a shortcut could be registered).
func openLog() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}

	path := filepath.Join(home, "Library", "Logs", "hopto.log")

	// A brand-new home has no Library/Logs folder yet.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return
	}

	log.SetOutput(file)
}
