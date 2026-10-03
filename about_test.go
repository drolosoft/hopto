package main

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// What the help panel shows for the ways a binary gets built.
func TestDescribeBuild(t *testing.T) {
	cases := []struct {
		name                     string
		version, commit, builtAt string
		want                     AboutView
	}{
		{
			name: "plain go build or wails dev",
			want: AboutView{Version: "dev"},
		},
		{
			name:    "tagged release",
			version: "v0.1.0",
			commit:  "0123456789abcdef0123456789abcdef01234567",
			builtAt: "2026-09-27T10:30:00Z",
			want: AboutView{
				Version: "v0.1.0", Commit: "0123456", BuiltAt: "2026-09-27",
			},
		},
		{
			name:    "work in progress",
			version: "v0.1.0-3-gabc1234-dirty",
			commit:  "abc1234",
			builtAt: "2026-09-27T23:59:59+02:00",
			want: AboutView{
				Version: "v0.1.0-3-gabc1234-dirty",
				Commit:  "abc1234",
				BuiltAt: "2026-09-27",
			},
		},
		{
			name:    "a date that is not RFC 3339 is left out",
			version: " v0.2.0 ",
			builtAt: "yesterday",
			want:    AboutView{Version: "v0.2.0"},
		},
	}

	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := describeBuild(test.version, test.commit, test.builtAt)
			if got != test.want {
				t.Errorf("got %+v, want %+v", got, test.want)
			}
		})
	}
}

// About adds the Go version and where the library file lives.
func TestAbout(t *testing.T) {
	app, _, _ := newTestApp(t)

	about := app.About()
	if about.GoVersion != runtime.Version() {
		t.Errorf("go = %q", about.GoVersion)
	}

	// The path uses the separator of the OS the test runs on.
	wantSuffix := filepath.Join("hopto", libraryFile)
	if !strings.HasSuffix(about.LibraryPath, wantSuffix) {
		t.Errorf("library = %q", about.LibraryPath)
	}

	if about.Version == "" {
		t.Error("version is empty")
	}
}
