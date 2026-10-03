package platform

// FileFilter limits a file dialog to one kind of file. It is this
// package's own type because platform must not import the Wails runtime;
// the window turns it into Wails' filter.
type FileFilter struct {
	// Name is what the dialog shows for the filter.
	Name string

	// Pattern is a semicolon separated list of globs, like "*.exe;*.lnk".
	Pattern string
}

// LoginAgent is the "Open at login" switch of the running system: a
// LaunchAgent on macOS, a value in the Run key on Windows. It is an
// interface so the App's tests can swap it for a fake.
type LoginAgent interface {
	// Enabled reports whether hopto is set to start at login.
	Enabled() bool

	// Enable sets hopto to start at login.
	Enable() error

	// Disable removes the login entry; one already gone is not an error.
	Disable() error
}
