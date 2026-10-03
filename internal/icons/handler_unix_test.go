//go:build !windows

package icons

import (
	"bytes"
	"net/http"
	"testing"
)

// This test reads an .icns as a bundle's icon, which only macOS does: on
// Windows the source of an app icon is its exe.

// A discovered app's icon comes out of its .icns through the resolver.
func TestHandlerServesBundleIcons(t *testing.T) {
	recorder := serve(
		t, t.TempDir(), http.MethodGet, RoutePrefix+"app-test.png",
	)

	body := recorder.Body.Bytes()
	isPNG := bytes.HasPrefix(body, []byte("\x89PNG"))
	if recorder.Code != http.StatusOK || !isPNG {
		t.Fatalf("status %d body %.8q", recorder.Code, body)
	}
}
