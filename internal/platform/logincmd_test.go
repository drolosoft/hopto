package platform

import "testing"

// An exe run from a temporary folder or a mounted zip would be gone at
// the next login.
func TestTemporaryExecutableRefused(t *testing.T) {
	for _, exe := range []string{
		`C:\Users\someone\AppData\Local\Temp\hopto\hopto.exe`,
		`C:\Users\someone\Downloads\hopto.zip\hopto.exe`,
		`C:\Windows\Temp\hopto.exe`,
	} {
		if !TemporaryExecutable(exe) {
			t.Errorf("%s: accepted", exe)
		}
	}

	programs := `C:\Users\someone\AppData\Local\Programs\hopto\hopto.exe`
	if TemporaryExecutable(programs) {
		t.Error("a Programs folder was refused")
	}
}

// The Run value quotes the path: Program Files has a space.
func TestQuotedCommand(t *testing.T) {
	got := QuotedCommand(`C:\Program Files\hopto\hopto.exe`)
	want := `"C:\Program Files\hopto\hopto.exe"`
	if got != want {
		t.Errorf("got %s", got)
	}
}
