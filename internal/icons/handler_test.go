package icons

import (
	"bytes"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// serve runs one request through a handler over dir, with resolve mapping
// "app-test" to the test icns.
func serve(
	t *testing.T, dir, method, path string,
) *httptest.ResponseRecorder {
	t.Helper()

	handler, err := NewHandler(dir, func(id string) (string, bool) {
		if id == "app-test" {
			return "testdata/edge-app.icns", true
		}

		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}

	// Windows will not remove a folder while the handler holds it open.
	t.Cleanup(func() { _ = handler.Close() })

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(method, path, nil))

	return recorder
}

// A user icon is served re-encoded and no larger than AppSide.
func TestHandlerServesUserIcons(t *testing.T) {
	dir := t.TempDir()
	iconPath := filepath.Join(dir, "mdn.png")
	if err := os.WriteFile(iconPath, solidPNG(t, 512), 0o600); err != nil {
		t.Fatal(err)
	}

	recorder := serve(t, dir, http.MethodGet, RoutePrefix+"mdn.png?v=123")

	contentType := recorder.Header().Get("Content-Type")
	if recorder.Code != http.StatusOK || contentType != "image/png" {
		t.Fatalf("status %d type %q", recorder.Code, contentType)
	}

	config, err := png.DecodeConfig(bytes.NewReader(recorder.Body.Bytes()))
	if err != nil || config.Width != AppSide {
		t.Errorf(
			"served %dx%d (%v), want %d",
			config.Width, config.Height, err, AppSide,
		)
	}

	if !strings.Contains(recorder.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("Cache-Control = %q", recorder.Header().Get("Cache-Control"))
	}
}

// Ids that are not ids, files that are not there, folders and other
// methods all end in 404 or 405, never in a file outside the icons folder.
func TestHandlerRefuses(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "folder.png"), 0o700); err != nil {
		t.Fatal(err)
	}

	outsidePath := filepath.Join(dir, "..", "outside.png")
	if err := os.WriteFile(outsidePath, solidPNG(t, 8), 0o600); err != nil {
		t.Fatal(err)
	}

	cases := map[string]int{
		RoutePrefix + "missing.png":      http.StatusNotFound,
		RoutePrefix + "folder.png":       http.StatusNotFound,
		RoutePrefix + "Upper.png":        http.StatusNotFound,
		RoutePrefix + "..%2Foutside.png": http.StatusNotFound,
		RoutePrefix + "../outside.png":   http.StatusNotFound,
		RoutePrefix + "mdn.jpg":          http.StatusNotFound,
		"/other/mdn.png":                 http.StatusNotFound,
		RoutePrefix:                      http.StatusNotFound,
	}

	for path, want := range cases {
		if got := serve(t, dir, http.MethodGet, path).Code; got != want {
			t.Errorf("GET %s = %d, want %d", path, got, want)
		}
	}

	postCode := serve(t, dir, http.MethodPost, RoutePrefix+"mdn.png").Code
	if postCode != http.StatusMethodNotAllowed {
		t.Errorf("POST = %d, want 405", postCode)
	}
}

// A file that is not an image is a 404, not a crash or raw bytes.
func TestHandlerRefusesNonImages(t *testing.T) {
	dir := t.TempDir()
	badPath := filepath.Join(dir, "bad.png")
	if err := os.WriteFile(badPath, []byte("<html>"), 0o600); err != nil {
		t.Fatal(err)
	}

	got := serve(t, dir, http.MethodGet, RoutePrefix+"bad.png").Code
	if got != http.StatusNotFound {
		t.Errorf("got %d, want 404", got)
	}
}

// FileURL carries the modification time so a refetched icon busts the
// WKWebView cache, and is empty for a missing file.
func TestFileURL(t *testing.T) {
	dir := t.TempDir()
	mdnPath := filepath.Join(dir, "mdn.png")
	if err := os.WriteFile(mdnPath, solidPNG(t, 8), 0o600); err != nil {
		t.Fatal(err)
	}

	got := FileURL(dir, "mdn")
	if !strings.HasPrefix(got, RoutePrefix+"mdn.png?v=") {
		t.Errorf("FileURL = %q", got)
	}

	if FileURL(dir, "missing") != "" || FileURL(dir, "../mdn") != "" {
		t.Error("a missing or invalid id must give an empty URL")
	}

	if StampURL("app-test", 42) != RoutePrefix+"app-test.png?v=42" {
		t.Errorf("StampURL = %q", StampURL("app-test", 42))
	}
}

// Once the handler is closed the folder can be removed, which is what
// Windows refuses while the handle is open, and closing again is harmless.
func TestHandlerCloseReleasesTheFolder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "icons")

	handler, err := NewHandler(dir, func(string) (string, bool) {
		return "", false
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := handler.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("removing the folder after Close: %v", err)
	}

	if err := handler.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
