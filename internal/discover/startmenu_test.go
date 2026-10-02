package discover

import (
	"os"
	"path/filepath"
	"testing"
)

// writeLink drops a shortcut under dir with a target and arguments.
func writeLink(t *testing.T, dir, name, target, args string) string {
	t.Helper()

	path := filepath.Join(dir, name+".lnk")
	data := linkBytes(target, args, "", 0, "")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

// writeFile drops a plain file at path.
func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Review Focus 1: uninstallers, documents, folders and duplicates stay
// out; a shortcut the parser cannot read stays in with its file name.
func TestScanStartMenuFilters(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "Foo")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	writeLink(t, sub, "Foo", `C:\Program Files\Foo\foo.exe`, "")
	writeLink(t, sub, "Uninstall Foo",
		`C:\Program Files\Foo\unins000.exe`, "")
	writeLink(t, sub, "Foo Manual", `C:\Program Files\Foo\manual.pdf`, "")
	writeLink(t, sub, "Foo Website", `C:\Program Files\Foo\site.url`, "")
	writeLink(t, root, "Foo again", `C:\Program Files\Foo\foo.exe`, "")
	writeFile(t, filepath.Join(root, "Broken.lnk"), "not a link")
	writeFile(t, filepath.Join(root, "notes.txt"), "x")

	apps := ScanStartMenu([]string{root}, "")

	names := map[string]bool{}
	for _, app := range apps {
		names[app.Name] = true
	}

	for _, want := range []string{"Broken", "Foo again"} {
		if !names[want] {
			t.Errorf("%s missing from %v", want, names)
		}
	}

	unwanted := []string{
		"Uninstall Foo", "Foo Manual", "Foo Website", "Foo", "notes",
	}
	for _, name := range unwanted {
		if names[name] {
			t.Errorf("%s should be out", name)
		}
	}
}

// A shortcut to msedge_proxy.exe with an app id is an Edge web app.
func TestEdgeShortcut(t *testing.T) {
	link := ShellLink{
		Target: `C:\Program Files (x86)\Microsoft\Edge\Application` +
			`\msedge_proxy.exe`,
		Args: `--profile-directory=Default --app-id=abcdef ` +
			`--app-url=https://example.org/`,
	}

	url, ok := isEdgeShortcut(link)
	if !ok || url != "https://example.org/" {
		t.Errorf("got %q %v", url, ok)
	}

	plain := ShellLink{Target: `C:\x\foo.exe`}
	if _, ok := isEdgeShortcut(plain); ok {
		t.Error("a plain exe counted as Edge")
	}
}

// Ids are stable and prefixed like the macOS ones.
func TestStartMenuIDs(t *testing.T) {
	root := t.TempDir()
	writeLink(t, root, "Zed Editor", `C:\Programs\zed.exe`, "")
	writeLink(t, root, "Alpha", `C:\Programs\alpha.exe`, "")

	apps := ScanStartMenu([]string{root}, "")
	if len(apps) != 2 || apps[0].ID != "app-alpha" ||
		apps[1].ID != "app-zed-editor" {
		t.Errorf("got %+v", apps)
	}
}
