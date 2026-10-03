//go:build !windows

package platform

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"howett.net/plist"

	"github.com/drolosoft/hopto/internal/atomicfile"
)

// launchAgentLabel names the job for launchd; it is the bundle id, so
// `launchctl list` shows whose job it is.
const launchAgentLabel = "com.drolosoft.hopto"

// launchAgentPerm is what launchd expects of a user agent: readable by
// everyone, writable by the owner only. A group-writable plist is
// refused at login.
const launchAgentPerm = 0o644

// errNotInBundle: the binary is not inside a .app (`go test`,
// `go build`). A login item pointing at a bare binary would start it
// without the bundle's Info.plist, as a Dock app, which is not hopto.
var errNotInBundle = errors.New("hopto is not running from a .app bundle")

// errTranslocated: the running .app sits under macOS's AppTranslocation
// folder. Gatekeeper puts a freshly downloaded, unnotarised app there, in
// a folder it recreates on every launch and empties on the next one, so a
// login item pointing at it would work once and then quietly fail after
// a reboot.
var errTranslocated = errors.New(
	"hopto is running from a translocated copy, not /Applications",
)

// translocationMarker is the folder name macOS's Gatekeeper interposes
// on the path of an app it has translocated.
const translocationMarker = "/AppTranslocation/"

// launchAgent is the plist launchd reads. `open` on the bundle instead
// of the binary makes the login start behave like a double click.
type launchAgent struct {
	// Label is the job's name for launchd, launchAgentLabel.
	Label string `plist:"Label"`

	// ProgramArguments is the command line launchd runs, program first.
	ProgramArguments []string `plist:"ProgramArguments"`

	// RunAtLoad starts the job as soon as launchd loads it, at login.
	RunAtLoad bool `plist:"RunAtLoad"`
}

// launchdLogin is the "Open at login" switch. It only writes the file and
// never registers it with launchctl: launchd reads the folder at the next
// login, so ticking the box never starts a second copy right now.
type launchdLogin struct {
	// path is the agent's plist under ~/Library/LaunchAgents.
	path string

	// executable is os.Executable in the app; a test passes its own so
	// it can pretend to run from a bundle.
	executable func() (string, error)
}

// launchAgentPath is where the user's agents live.
func launchAgentPath(home string) string {
	return filepath.Join(
		home, "Library", "LaunchAgents", launchAgentLabel+".plist",
	)
}

// launchAgentPlist encodes the agent for a bundle path as XML, the
// format launchd documents and a person can read.
func launchAgentPlist(bundle string) ([]byte, error) {
	agent := launchAgent{
		Label:            launchAgentLabel,
		ProgramArguments: []string{"/usr/bin/open", bundle},
		RunAtLoad:        true,
	}

	return plist.MarshalIndent(agent, plist.XMLFormat, "\t")
}

// bundlePath climbs from the running binary to its .app: the binary of
// a bundle is always <name>.app/Contents/MacOS/<binary>.
func bundlePath(executable string) (string, error) {
	macOS := filepath.Dir(executable)
	contents := filepath.Dir(macOS)
	bundle := filepath.Dir(contents)

	inBundle := filepath.Base(macOS) == "MacOS" &&
		filepath.Base(contents) == "Contents" &&
		strings.HasSuffix(bundle, ".app")
	if !inBundle {
		return "", fmt.Errorf("%w: %s", errNotInBundle, executable)
	}

	if strings.Contains(bundle, translocationMarker) {
		return "", fmt.Errorf("%w: %s", errTranslocated, bundle)
	}

	return bundle, nil
}

// Enabled reports whether the agent file is there.
func (login launchdLogin) Enabled() bool {
	_, err := os.Stat(login.path)
	return err == nil
}

// Enable writes the agent for the running bundle.
func (login launchdLogin) Enable() error {
	executable, err := login.executable()
	if err != nil {
		return err
	}

	bundle, err := bundlePath(executable)
	if err != nil {
		return err
	}

	data, err := launchAgentPlist(bundle)
	if err != nil {
		return err
	}

	return atomicfile.Write(login.path, data, launchAgentPerm)
}

// Disable removes the agent; a file already gone is not an error.
func (login launchdLogin) Disable() error {
	err := os.Remove(login.path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}
