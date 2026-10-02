package seed

import (
	"bytes"
	"strings"
	"testing"
)

// withHotkeys only touches the two shortcut lines; the links and the
// categories of the seed stay byte for byte.
func TestWithHotkeysRewritesOnlyTheTwoLines(t *testing.T) {
	out := string(withHotkeys(english, "ctrl+shift+space", "ctrl+alt+space"))

	if !strings.Contains(out, "hotkey_apps = \"ctrl+shift+space\"\n") {
		t.Error("hotkey_apps was not rewritten")
	}

	if !strings.Contains(out, "hotkey_links = \"ctrl+alt+space\"\n") {
		t.Error("hotkey_links was not rewritten")
	}

	if strings.Contains(out, "cmd+") {
		t.Error("a cmd shortcut survived")
	}

	wantLines := strings.Count(string(english), "\n")
	if got := strings.Count(out, "\n"); got != wantLines {
		t.Errorf("line count changed: %d, was %d", got, wantLines)
	}
}

// A Windows checkout with autocrlf carries CRLF endings; the two lines
// must still be rewritten and every line must keep its \r\n.
func TestWithHotkeysKeepsCRLFEndings(t *testing.T) {
	crlf := bytes.ReplaceAll(english, []byte("\n"), []byte("\r\n"))
	out := string(withHotkeys(crlf, "ctrl+shift+space", "ctrl+alt+space"))

	if !strings.Contains(out, "hotkey_apps = \"ctrl+shift+space\"\r\n") {
		t.Error("hotkey_apps was not rewritten")
	}

	if !strings.Contains(out, "hotkey_links = \"ctrl+alt+space\"\r\n") {
		t.Error("hotkey_links was not rewritten")
	}

	if strings.Contains(out, "cmd+") {
		t.Error("a cmd shortcut survived")
	}

	lines := strings.Count(out, "\n")
	if got := strings.Count(out, "\r\n"); got != lines {
		t.Errorf("%d CRLF endings for %d lines", got, lines)
	}
}
