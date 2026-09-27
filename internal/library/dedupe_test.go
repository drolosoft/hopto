package library

import "testing"

// Two spellings of the same page compare equal; different pages do not.
func TestNormalizeURL(t *testing.T) {
	same := []string{
		"https://github.com",
		"HTTPS://GitHub.com/",
		"https://www.github.com/#readme",
		"https://github.com:443/",
	}

	for _, raw := range same {
		if got := NormalizeURL(raw); got != "https://github.com" {
			t.Errorf("NormalizeURL(%q) = %q", raw, got)
		}
	}

	deeper := NormalizeURL("https://github.com/drolosoft")
	if deeper == NormalizeURL("https://github.com") {
		t.Fatal("different paths compared equal")
	}

	if NormalizeURL("http://github.com") == NormalizeURL("https://github.com") {
		t.Fatal("http and https compared equal")
	}
}

// An exact duplicate is found; a link on the same host is only a neighbour.
func TestFindDuplicateAndNeighbours(t *testing.T) {
	links := []Link{
		{ID: "github", URL: "https://github.com"},
		{ID: "drolosoft-gh", URL: "https://github.com/drolosoft"},
	}

	duplicate, ok := FindDuplicateLink(links, "https://www.github.com/")
	if !ok || duplicate.ID != "github" {
		t.Fatalf("duplicate = %+v ok=%v", duplicate, ok)
	}

	sameOther := "https://github.com/drolosoft/hopto"
	if _, ok := FindDuplicateLink(links, sameOther); ok {
		t.Fatal("a different path was reported as duplicate")
	}

	if neighbours := LinksOnSameHost(links, sameOther); len(neighbours) != 2 {
		t.Fatalf("neighbours = %d, want 2", len(neighbours))
	}
}
