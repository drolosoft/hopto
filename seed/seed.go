// Package seed holds the library hopto writes on its first run: a handful of
// generic links and three categories, enough to show what the panel does
// before the user adds anything. Nothing personal belongs here, and no
// images either: the links carry icon hints and hopto fetches the icons
// the first time it shows them.
package seed

import _ "embed"

// The two seeds, one per language of the interface. They list the same
// links with the same ids; only the texts differ.
var (
	//go:embed seed.toml
	english []byte

	//go:embed seed.es.toml
	spanish []byte
)

// For returns the seed for a language; anything but "es" gets English.
func For(language string) []byte {
	if language == "es" {
		return platformSeed(spanish)
	}

	return platformSeed(english)
}
