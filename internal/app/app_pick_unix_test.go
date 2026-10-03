//go:build !windows

package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These tests pick .app bundles inside and outside the folders macOS
// installs apps in, so they run everywhere but Windows. The dialog tests
// that need no bundle stay in app_pick_test.go.

// looseBundle writes a minimal .app somewhere outside the app roots.
func looseBundle(t *testing.T, name string) string {
	t.Helper()

	bundle := filepath.Join(t.TempDir(), "Downloads", name+".app")
	contents := filepath.Join(bundle, "Contents")
	if err := os.MkdirAll(contents, 0o755); err != nil {
		t.Fatal(err)
	}

	plist := `<?xml version="1.0" encoding="UTF-8"?><plist version="1.0">` +
		`<dict><key>CFBundleIdentifier</key>` +
		`<string>com.example.loose</string></dict></plist>`
	infoPath := filepath.Join(contents, "Info.plist")
	if err := os.WriteFile(infoPath, []byte(plist), 0o644); err != nil {
		t.Fatal(err)
	}

	return bundle
}

// Cancelling the panel answers an empty draft, and the window floats
// again afterwards, with the focus back.
func TestPickAppWithdrawsAlwaysOnTop(t *testing.T) {
	app, win, _ := newTestApp(t)
	app.visible = true

	draft, err := app.PickApp()
	if err != nil || draft.Path != "" {
		t.Fatalf("cancel: %+v %v", draft, err)
	}

	want := "ontop:false pick:/Applications ontop:true activate"
	if got := win.joined(); got != want {
		t.Errorf("calls = %q, want %q", got, want)
	}
}

// A picked bundle comes back with its name, bundle id and icon, and a
// discovered copy of the same app is named as the duplicate.
func TestPickAppReadsTheBundle(t *testing.T) {
	app, win, _ := newTestApp(t)
	app.visible = true
	win.pickPath = fakeBundle(t, app, "Alpha", "com.example.alpha") + "/"

	draft, err := app.PickApp()
	if err != nil {
		t.Fatal(err)
	}

	wrong := draft.Name != "Alpha" ||
		draft.BundleID != "com.example.alpha" ||
		strings.HasSuffix(draft.Path, "/") ||
		draft.Problem != "" ||
		!strings.HasPrefix(draft.IconDataURL, "data:image/png;base64,")
	if wrong {
		t.Errorf("draft = %+v", draft)
	}

	if draft.Duplicate == nil || draft.Duplicate.ID != "app-alpha" {
		t.Errorf("duplicate = %+v", draft.Duplicate)
	}
}

// A bundle outside /Applications, /System/Applications and
// ~/Applications is reported, not silently accepted.
func TestPickAppOutsideTheRoots(t *testing.T) {
	app, win, _ := newTestApp(t)
	app.visible = true
	win.pickPath = looseBundle(t, "Loose")

	draft, err := app.PickApp()
	if err != nil {
		t.Fatal(err)
	}

	if draft.Problem != "app.path" || draft.Name != "Loose" {
		t.Errorf("draft = %+v", draft)
	}

	if draft.Duplicate != nil {
		t.Errorf("duplicate = %+v", draft.Duplicate)
	}
}
