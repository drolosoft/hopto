//go:build !windows

package seed_test

// config.example.toml is the macOS seed with its Command shortcuts, so
// this test runs everywhere but Windows, whose seed differs in them.

import (
	"os"
	"reflect"
	"testing"

	"github.com/drolosoft/hopto/internal/library"
	"github.com/drolosoft/hopto/internal/platform"
	"github.com/drolosoft/hopto/seed"
)

// config.example.toml is the English seed with comments.
// It must stay a library hopto accepts, with no key hopto would ignore
// (a typo there would teach the typo), and hold exactly what a first
// run writes, so the documented example and the real one never drift.
func TestConfigExampleIsTheSeed(t *testing.T) {
	data, err := os.ReadFile("../config.example.toml")
	if err != nil {
		t.Fatal(err)
	}

	example, err := library.Decode(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if err := library.Validate(example, "/Users/someone"); err != nil {
		t.Errorf("validate: %v", err)
	}

	if unknown := library.UnknownKeys(data); len(unknown) > 0 {
		t.Errorf("keys hopto does not read: %v", unknown)
	}

	first, err := library.Decode(seed.For(platform.LanguageEnglish))
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(example, first) {
		t.Errorf("example and English seed differ:\n%+v\n%+v", example, first)
	}
}
