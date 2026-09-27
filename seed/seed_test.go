package seed

import (
	"testing"

	"github.com/drolosoft/hopto/internal/library"
)

// Both seeds must be valid libraries with the same ids: they are the first
// file a new user gets, and the ids are what usage keys and icon files
// hang from, so switching language must not change them.
func TestSeedsAreValidAndAligned(t *testing.T) {
	english, err := library.Decode(For("en"))
	if err != nil {
		t.Fatal(err)
	}

	spanish, err := library.Decode(For("es"))
	if err != nil {
		t.Fatal(err)
	}

	byLanguage := map[string]library.Library{"en": english, "es": spanish}
	for name, lib := range byLanguage {
		if err := library.Validate(lib, "/Users/someone"); err != nil {
			t.Fatalf("%s: %v", name, err)
		}

		if len(lib.Links) < 5 || len(lib.Categories) != 3 {
			t.Fatalf(
				"%s: seed has %d links and %d categories",
				name, len(lib.Links), len(lib.Categories),
			)
		}
	}

	for index, link := range english.Links {
		other := spanish.Links[index]
		if other.ID != link.ID || other.URL != link.URL {
			t.Errorf(
				"link %d differs between languages: %s vs %s",
				index, link.ID, other.ID,
			)
		}
	}

	for index, category := range english.Categories {
		if spanish.Categories[index].ID != category.ID {
			t.Errorf("category %d differs between languages", index)
		}
	}

	if string(For("fr")) != string(For("en")) {
		t.Error("an unknown language must fall back to English")
	}
}
