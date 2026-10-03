package platform

// WindowClassName is the Win32 class of hopto's window, set through
// Wails' options so the native side can find the window by name. The
// options are built the same way on every system, so the name is shared;
// macOS ignores it.
const WindowClassName = "hoptoWindow"
