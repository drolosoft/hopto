// Package seed holds the library hopto writes on its first run: a handful of
// generic links and three categories, enough to show what the panel does
// before the user adds anything. Nothing personal belongs here.
package seed

import _ "embed"

// Library is seed.toml as bytes; internal/library parses it.
//
//go:embed seed.toml
var Library []byte
