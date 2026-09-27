package library

import "testing"

// Names become slugs the way a person would write them by hand.
func TestSlug(t *testing.T) {
	// Slug caps at 64 characters; this checks the cut lands on a dash
	// instead of splitting a word in half.
	longInput := "a-very-long-name-that-goes-on-and-on-and-on-and-on-and-on-" +
		"and-on-and-on-and-on"
	truncated := "a-very-long-name-that-goes-on-and-on-and-on-and-on-and-on-" +
		"and-on"

	cases := map[string]string{
		"GitHub":               "github",
		"La Porra · Champions": "la-porra-champions",
		"Cronómetro":           "cronometro",
		"  Outlook (PWA)  ":    "outlook-pwa",
		"!!!":                  "item",
		"":                     "item",
		"edge-something":       "item-edge-something",
		longInput:              truncated,
	}

	for name, want := range cases {
		if got := Slug(name); got != want {
			t.Errorf("Slug(%q) = %q, want %q", name, got, want)
		}
	}
}

// A taken id gets a numeric suffix, counting up until one is free.
func TestUniqueID(t *testing.T) {
	taken := map[string]bool{"github": true, "github-2": true}
	isTaken := func(id string) bool { return taken[id] }

	if got := UniqueID("github", isTaken); got != "github-3" {
		t.Fatalf("got %q, want github-3", got)
	}

	if got := UniqueID("mdn", isTaken); got != "mdn" {
		t.Fatalf("got %q, want mdn", got)
	}
}
