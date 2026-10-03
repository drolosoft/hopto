//go:build windows

package platform

import "testing"

// The Windows folders are pinned to their exact paths: the data under
// Roaming, the log under Local.
func TestWindowsPathsAreExact(t *testing.T) {
	home := `C:\Users\someone`

	wantData := `C:\Users\someone\AppData\Roaming\hopto`
	if got := DataDir(home); got != wantData {
		t.Errorf("DataDir = %q, want %q", got, wantData)
	}

	wantLog := `C:\Users\someone\AppData\Local\hopto\hopto.log`
	if got := logPath(home); got != wantLog {
		t.Errorf("logPath = %q, want %q", got, wantLog)
	}
}
