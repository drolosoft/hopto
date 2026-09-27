package discover

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// plistFor writes the smallest Info.plist with the given keys.
func plistFor(identifier, iconFile string) string {
	var builder strings.Builder
	builder.WriteString(
		`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict>`,
	)

	if identifier != "" {
		builder.WriteString(
			"<key>CFBundleIdentifier</key><string>" + identifier + "</string>",
		)
	}

	if iconFile != "" {
		builder.WriteString(
			"<key>CFBundleIconFile</key><string>" + iconFile + "</string>",
		)
	}

	builder.WriteString("</dict></plist>")

	return builder.String()
}

// fakeBundle creates <dir>/<name>.app with an Info.plist and, when
// iconFile is set, a copy of the test icns under Resources.
func fakeBundle(t *testing.T, dir, name, plist, iconFile string) string {
	t.Helper()

	bundle := filepath.Join(dir, name+".app")
	resources := filepath.Join(bundle, "Contents", "Resources")

	if err := os.MkdirAll(resources, 0o755); err != nil {
		t.Fatal(err)
	}

	if plist != "" {
		infoPath := filepath.Join(bundle, "Contents", "Info.plist")
		if err := os.WriteFile(infoPath, []byte(plist), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if iconFile != "" {
		icns, err := os.ReadFile("../icons/testdata/edge-app.icns")
		if err != nil {
			t.Fatal(err)
		}

		iconPath := filepath.Join(resources, iconFile)
		if err := os.WriteFile(iconPath, icns, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	return bundle
}

// ids lists the ids of a scan, joined for easy comparison.
func ids(apps []App) string {
	out := make([]string, 0, len(apps))
	for _, app := range apps {
		out = append(out, app.ID)
	}

	return strings.Join(out, " ")
}

// An Edge web app: the folder name is the name, the plist gives the URL
// (shown as host) and the icon.
func TestInspectBundleReadsAnEdgeApp(t *testing.T) {
	plist, err := os.ReadFile("testdata/edge-info.plist")
	if err != nil {
		t.Fatal(err)
	}

	bundle := fakeBundle(
		t, t.TempDir(), "Outlook (PWA)", string(plist), "app.icns",
	)

	app, err := InspectBundle(bundle)
	if err != nil {
		t.Fatal(err)
	}

	if app.URL != "https://example.com/" ||
		app.Name != "Outlook (PWA)" ||
		app.Source != SourceEdge ||
		app.Host != "example.com" ||
		app.Description != "example.com" {
		t.Errorf("app = %+v", app)
	}

	iconPath := filepath.Join(bundle, "Contents", "Resources", "app.icns")
	if app.IconPath != iconPath {
		t.Errorf("icon = %q", app.IconPath)
	}
}

// A regular app: CFBundleIconFile may or may not carry the extension; a
// bundle without CFBundleIconFile (icons in Assets.car) has no icon path.
func TestInspectBundleReadsARegularApp(t *testing.T) {
	dir := t.TempDir()

	withExt := fakeBundle(
		t, dir, "Alpha", plistFor("com.example.alpha", "Alpha.icns"), "Alpha.icns",
	)
	noExt := fakeBundle(
		t, dir, "Beta", plistFor("com.example.beta", "Beta"), "Beta.icns",
	)
	noIcon := fakeBundle(t, dir, "Gamma", plistFor("com.example.gamma", ""), "")
	iconMissing := fakeBundle(
		t, dir, "Delta", plistFor("com.example.delta", "Delta"), "",
	)

	cases := map[string]string{
		withExt:     filepath.Join(withExt, "Contents", "Resources", "Alpha.icns"),
		noExt:       filepath.Join(noExt, "Contents", "Resources", "Beta.icns"),
		noIcon:      "",
		iconMissing: "",
	}

	for bundle, wantIcon := range cases {
		app, err := InspectBundle(bundle)
		if err != nil {
			t.Fatal(err)
		}

		isApplications := app.Source == SourceApplications
		hasPrefix := strings.HasPrefix(app.BundleID, "com.example.")
		if app.IconPath != wantIcon || !isApplications || !hasPrefix {
			t.Errorf("%s: %+v", bundle, app)
		}
	}
}

// No plist, or a plist that does not parse: the name survives, the error
// says why, and nothing else is claimed.
func TestInspectBundleSurvivesBrokenBundles(t *testing.T) {
	dir := t.TempDir()
	noPlist := fakeBundle(t, dir, "NoPlist", "", "")
	badPlist := fakeBundle(t, dir, "BadPlist", "not a plist", "")

	for _, bundle := range []string{noPlist, badPlist} {
		app, err := InspectBundle(bundle)
		if err == nil {
			t.Errorf("%s: expected an error", bundle)
		}

		wantName := strings.TrimSuffix(filepath.Base(bundle), ".app")
		if app.Name != wantName || app.BundleID != "" || app.IconPath != "" {
			t.Errorf("%s: %+v", bundle, app)
		}
	}

	if _, err := InspectBundle(filepath.Join(dir, "nope.app")); err == nil {
		t.Error("a missing bundle gave no error")
	}
}

func TestScanEdgeApps(t *testing.T) {
	plist, _ := os.ReadFile("testdata/edge-info.plist")
	dir := t.TempDir()
	fakeBundle(t, dir, "Outlook (PWA)", string(plist), "app.icns")
	fakeBundle(t, dir, "Música", string(plist), "app.icns")

	apps := ScanEdgeApps(dir)

	if got := ids(apps); got != "edge-musica edge-outlook-pwa" {
		t.Errorf("ids = %q", got)
	}

	if apps[0].Source != SourceEdge || apps[0].Description != "example.com" {
		t.Errorf("app = %+v", apps[0])
	}

	missing := ScanEdgeApps(filepath.Join(dir, "nope"))
	if missing == nil || len(missing) != 0 {
		t.Errorf("missing folder: %v", missing)
	}
}

// Review Focus 3: a bundle with an unreadable plist stays in the list with
// its folder name and no icon; the rest of the scan is unaffected.
func TestScanApplicationsSurvivesABrokenBundle(t *testing.T) {
	root := t.TempDir()
	fakeBundle(
		t, root, "Alpha", plistFor("com.example.alpha", "Alpha"), "Alpha.icns",
	)
	fakeBundle(t, root, "Broken", "garbage", "")

	apps := ScanApplications([]string{root})

	if got := ids(apps); got != "app-alpha app-broken" {
		t.Fatalf("ids = %q", got)
	}

	broken := apps[1]
	if broken.Name != "Broken" ||
		broken.IconPath != "" ||
		broken.BundleID != "" {
		t.Errorf("broken = %+v", broken)
	}
}

// Utilities and *.localized are scanned one level down, browser PWA
// folders are left to the Edge scan, plain files and folders are skipped,
// and the result is sorted by name regardless of folder.
func TestScanApplicationsWalksTheKnownSubfolders(t *testing.T) {
	root := t.TempDir()
	utilities := filepath.Join(root, "Utilities")
	extras := filepath.Join(root, "Extras.localized")
	edgeApps := filepath.Join(root, "Edge Apps.localized")
	nested := filepath.Join(root, "Deeper", "Nested")

	fakeBundle(t, root, "Zeta", plistFor("com.example.zeta", ""), "")
	fakeBundle(
		t, utilities, "Terminal", plistFor("com.example.terminal", ""), "",
	)
	fakeBundle(t, extras, "Extra", plistFor("com.example.extra", ""), "")
	fakeBundle(t, edgeApps, "Outlook", plistFor("com.example.outlook", ""), "")
	fakeBundle(t, nested, "Hidden", plistFor("com.example.hidden", ""), "")

	notesPath := filepath.Join(root, "notes.app")
	if err := os.WriteFile(notesPath, []byte("a file"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := ids(ScanApplications([]string{root}))
	if got != "app-extra app-terminal app-zeta" {
		t.Errorf("ids = %q", got)
	}
}

// The same app installed in two roots (or reachable through a symlink) is
// listed once, from the first root; two different apps with the same name
// get distinct ids.
func TestScanApplicationsDeduplicates(t *testing.T) {
	system := t.TempDir()
	user := t.TempDir()
	fakeBundle(t, system, "Alpha", plistFor("com.example.alpha", ""), "")
	fakeBundle(t, user, "Alpha", plistFor("com.example.alpha", ""), "")
	fakeBundle(t, user, "Beta", plistFor("com.example.beta", ""), "")
	fakeBundle(t, system, "Beta", plistFor("com.example.beta-other", ""), "")

	systemBeta := filepath.Join(system, "Beta.app")
	userBetaLink := filepath.Join(user, "Beta link.app")
	if err := os.Symlink(systemBeta, userBetaLink); err != nil {
		t.Fatal(err)
	}

	apps := ScanApplications([]string{system, user})

	if got := ids(apps); got != "app-alpha app-beta app-beta-2" {
		t.Errorf("ids = %q", got)
	}

	if apps[0].Path != filepath.Join(system, "Alpha.app") {
		t.Errorf("first root must win: %q", apps[0].Path)
	}
}

// The scanner only walks again when a folder changed.
func TestScannerCachesByFolderTime(t *testing.T) {
	root := t.TempDir()
	fakeBundle(t, root, "Alpha", plistFor("com.example.alpha", ""), "")

	scanner := &Scanner{}
	first := scanner.Applications([]string{root})
	if ids(first) != "app-alpha" {
		t.Fatalf("first = %q", ids(first))
	}

	// Same folder time: the cached list is returned even though a bundle
	// appeared (that is the trade-off of the mtime cache; a real install
	// always bumps the folder time).
	stat, _ := os.Stat(root)
	fakeBundle(t, root, "Beta", plistFor("com.example.beta", ""), "")
	if err := os.Chtimes(root, stat.ModTime(), stat.ModTime()); err != nil {
		t.Fatal(err)
	}

	if got := ids(scanner.Applications([]string{root})); got != "app-alpha" {
		t.Errorf("cache miss without a folder change: %q", got)
	}

	// A newer folder time invalidates it.
	later := stat.ModTime().Add(2 * 1e9)
	if err := os.Chtimes(root, later, later); err != nil {
		t.Fatal(err)
	}

	afterChange := ids(scanner.Applications([]string{root}))
	if afterChange != "app-alpha app-beta" {
		t.Errorf("after change: %q", afterChange)
	}

	// The caller gets a copy: editing it does not poison the cache.
	scanner.Applications([]string{root})[0].Name = "changed"
	if scanner.Applications([]string{root})[0].Name != "Alpha" {
		t.Error("the cache was mutated through the returned slice")
	}
}

// Three hundred bundles, the size of a busy /Applications, must scan well
// under the 50 ms budget the spec gives the whole appearance.
func BenchmarkScanApplications(b *testing.B) {
	root := b.TempDir()
	for index := range 300 {
		name := "App" + strings.Repeat("x", index%7) + string(rune('a'+index%26))
		bundle := filepath.Join(root, name+string(rune('0'+index%10))+".app")
		resources := filepath.Join(bundle, "Contents", "Resources")
		infoPath := filepath.Join(bundle, "Contents", "Info.plist")
		identifier := "com.example.b" + string(rune('a'+index%26))

		_ = os.MkdirAll(resources, 0o755)
		_ = os.WriteFile(infoPath, []byte(plistFor(identifier, "")), 0o644)
	}

	b.ResetTimer()
	for range b.N {
		ScanApplications([]string{root})
	}
}
