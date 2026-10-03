//go:build windows

package main

import (
	"errors"
	"fmt"

	"github.com/drolosoft/hopto/internal/platform"
	"golang.org/x/sys/windows/registry"
)

// runKey is where Windows lists what starts with the user's session.
const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// runValue is hopto's own entry in it.
const runValue = "hopto"

// errTemporary answers an exe that would not be there at login.
var errTemporary = errors.New("hopto is running from a temporary folder")

// loginAgent is the "Open at login" switch on Windows: a value in the
// user's Run key. executable is injectable so the tests never read
// the real one.
type loginAgent struct {
	executable func() (string, error)
}

// openRunKey opens the key for reading and writing; it always exists.
func openRunKey() (registry.Key, error) {
	return registry.OpenKey(
		registry.CURRENT_USER, runKey,
		registry.QUERY_VALUE|registry.SET_VALUE,
	)
}

// Enabled reports whether the value is there.
func (l loginAgent) Enabled() bool {
	key, err := openRunKey()
	if err != nil {
		return false
	}
	defer func() { _ = key.Close() }()

	_, _, err = key.GetStringValue(runValue)

	return err == nil
}

// Enable writes the value for the running exe.
func (l loginAgent) Enable() error {
	exe, err := l.executable()
	if err != nil {
		return err
	}

	if platform.TemporaryExecutable(exe) {
		return fmt.Errorf("%w: %s", errTemporary, exe)
	}

	key, err := openRunKey()
	if err != nil {
		return err
	}
	defer func() { _ = key.Close() }()

	return key.SetStringValue(runValue, platform.QuotedCommand(exe))
}

// Disable removes the value; a value that is not there is fine.
func (l loginAgent) Disable() error {
	key, err := openRunKey()
	if err != nil {
		return err
	}
	defer func() { _ = key.Close() }()

	err = key.DeleteValue(runValue)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}

	return err
}
