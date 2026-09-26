package seed

import (
	"testing"

	"github.com/drolosoft/hopto/internal/library"
)

// The embedded seed must always be a valid library: it is the first file a
// new user gets.
func TestSeedIsValid(t *testing.T) {
	lib, err := library.Decode(Library)
	if err != nil {
		t.Fatal(err)
	}

	if err := library.Validate(lib, "/Users/someone"); err != nil {
		t.Fatal(err)
	}

	if len(lib.Links) < 5 || len(lib.Categories) != 3 {
		t.Fatalf("seed has %d links and %d categories", len(lib.Links), len(lib.Categories))
	}
}
