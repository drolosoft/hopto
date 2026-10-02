//go:build !windows

package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

// The plist launchd reads: XML, the label, RunAtLoad, and `open` on the
// .app, so a login start goes through Launch Services like a double
// click and never runs the binary inside the bundle.
func TestLaunchAgentPlist(t *testing.T) {
	data, err := launchAgentPlist("/Applications/hopto.app")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(string(data), "<?xml") {
		t.Errorf("not XML: %.40q", data)
	}

	var agent struct {
		Label            string   `plist:"Label"`
		ProgramArguments []string `plist:"ProgramArguments"`
		RunAtLoad        bool     `plist:"RunAtLoad"`
	}
	if _, err := plist.Unmarshal(data, &agent); err != nil {
		t.Fatal(err)
	}

	arguments := strings.Join(agent.ProgramArguments, " ")
	wrong := agent.Label != launchAgentLabel ||
		arguments != "/usr/bin/open /Applications/hopto.app" ||
		!agent.RunAtLoad
	if wrong {
		t.Errorf("agent = %+v", agent)
	}
}

// Only a binary inside Contents/MacOS of a .app has a bundle; `go test`
// and a bare `go build` do not, and must say so.
func TestBundlePath(t *testing.T) {
	cases := []struct {
		executable string
		want       string
	}{
		{
			"/Applications/hopto.app/Contents/MacOS/hopto",
			"/Applications/hopto.app",
		},
		{
			"/Users/someone/Applications/hopto.app/Contents/MacOS/hopto",
			"/Users/someone/Applications/hopto.app",
		},
		{"/private/var/folders/x/b001/hopto.test", ""},
		{"/Users/someone/go/bin/hopto", ""},
		{"/tmp/hopto.app/hopto", ""},
	}

	for _, test := range cases {
		got, err := bundlePath(test.executable)
		if got != test.want {
			t.Errorf("%s: got %q, want %q", test.executable, got, test.want)
		}

		if test.want == "" && !errors.Is(err, errNotInBundle) {
			t.Errorf("%s: err = %v", test.executable, err)
		}
	}
}

// Gatekeeper runs a freshly downloaded, unnotarised copy from a folder
// under /AppTranslocation/ that macOS recreates on every launch and
// empties on the next one: a login item pointing there would work once
// and then silently stop, so bundlePath must refuse it outright.
func TestBundlePathRefusesTranslocation(t *testing.T) {
	const executable = "/private/var/folders/x/AppTranslocation/" +
		"1234-5678/d/hopto.app/Contents/MacOS/hopto"

	got, err := bundlePath(executable)
	if got != "" {
		t.Errorf("got %q, want no bundle path", got)
	}

	if !errors.Is(err, errTranslocated) {
		t.Errorf("err = %v, want errTranslocated", err)
	}
}

// Enable writes the plist, Disable removes it, twice is harmless, and
// outside a bundle nothing is written at all.
func TestLoginAgentEnableDisable(t *testing.T) {
	home := t.TempDir()
	executable := "/Applications/hopto.app/Contents/MacOS/hopto"
	agent := loginAgent{
		path:       launchAgentPath(home),
		executable: func() (string, error) { return executable, nil },
	}

	if agent.Enabled() {
		t.Fatal("enabled before anything was written")
	}

	if err := agent.Enable(); err != nil || !agent.Enabled() {
		t.Fatalf("enable: %v", err)
	}

	info, err := os.Stat(agent.path)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Errorf("file: %v %v", info, err)
	}

	for range 2 {
		if err := agent.Disable(); err != nil || agent.Enabled() {
			t.Fatalf("disable: %v", err)
		}
	}

	executable = filepath.Join(t.TempDir(), "hopto.test")
	if err := agent.Enable(); !errors.Is(err, errNotInBundle) {
		t.Errorf("outside a bundle: %v", err)
	}

	if agent.Enabled() {
		t.Error("a plist was written outside a bundle")
	}
}
