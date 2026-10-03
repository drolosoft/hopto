package app

import (
	"runtime"
	"strings"
	"time"
)

// The build stamps, set by `make build` through -ldflags
// "-X github.com/drolosoft/hopto/internal/app.…". A plain `go build` or
// `wails dev` leaves them empty and the help panel says "dev".
var (
	version = ""
	commit  = ""
	builtAt = ""
)

// shortCommit is how much of a commit hash git itself shows by default,
// enough to find the commit and short enough for one line of help.
const shortCommit = 7

// AboutView is what the help panel says about the running build.
type AboutView struct {
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	BuiltAt     string `json:"builtAt"`
	GoVersion   string `json:"goVersion"`
	LibraryPath string `json:"libraryPath"`
}

// About describes the build and says where library.toml lives, so a bug
// report can name both without opening a terminal.
func (a *App) About() AboutView {
	view := describeBuild(version, commit, builtAt)
	view.GoVersion = runtime.Version()
	view.LibraryPath = a.library.Path()

	return view
}

// describeBuild tidies the stamps: an unstamped build is "dev", the
// commit is cut to the length git shows, and the date keeps only the day
// of an RFC 3339 stamp; anything else is dropped rather than shown half
// understood.
func describeBuild(rawVersion, rawCommit, rawBuiltAt string) AboutView {
	view := AboutView{
		Version: strings.TrimSpace(rawVersion),
		Commit:  strings.TrimSpace(rawCommit),
	}

	if view.Version == "" {
		view.Version = "dev"
	}

	if len(view.Commit) > shortCommit {
		view.Commit = view.Commit[:shortCommit]
	}

	stamp, err := time.Parse(time.RFC3339, strings.TrimSpace(rawBuiltAt))
	if err == nil {
		view.BuiltAt = stamp.Format(time.DateOnly)
	}

	return view
}
