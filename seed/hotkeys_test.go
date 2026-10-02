package seed

import (
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
