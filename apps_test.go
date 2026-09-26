package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveFindsBundlesInTheRepository builds a fake checkout with one app
// built and one missing, and checks that only the built one gets a path.
func TestResolveFindsBundlesInTheRepository(t *testing.T) {
	repo := t.TempDir()
	built := filepath.Join(repo, "cronometro", "build", "bin", "cronometro.app")

	if err := os.MkdirAll(built, 0o755); err != nil {
		t.Fatal(err)
	}

	apps := resolveCatalog(repo)

	if len(apps) != len(catalog) {
		t.Fatalf("got %d apps, want %d", len(apps), len(catalog))
	}

	if apps[0].Path != built {
		t.Errorf("cronometro path = %q, want %q", apps[0].Path, built)
	}

	if apps[1].Path != "" {
		t.Errorf("cunyas path = %q, want empty (not built)", apps[1].Path)
	}
}

// TestResolveIgnoresFilesNamedLikeBundles makes sure a plain file does not
// pass as an app bundle.
func TestResolveIgnoresFilesNamedLikeBundles(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, "cronometro", "build", "bin")

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "cronometro.app"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	if apps := resolveCatalog(repo); apps[0].Path != "" {
		t.Errorf("a file was accepted as a bundle: %q", apps[0].Path)
	}
}

// TestRepoRootFromBundlePath checks the walk from the binary inside the built
// bundle up to the checkout, and that a path outside a checkout yields "".
func TestRepoRootFromBundlePath(t *testing.T) {
	repo := t.TempDir()
	binary := filepath.Join(repo, "launcher", "build", "bin", "launcher.app", "Contents", "MacOS", "launcher")

	if err := os.MkdirAll(filepath.Dir(binary), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(repo, "launcher", "go.mod"), []byte("module launcher\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := repoRootFrom(binary); got != repo {
		t.Errorf("repoRootFrom = %q, want %q", got, repo)
	}

	if got := repoRootFrom("/Applications/hopto-apps/launcher.app/Contents/MacOS/launcher"); got != "" {
		t.Errorf("outside a checkout got %q, want empty", got)
	}
}
