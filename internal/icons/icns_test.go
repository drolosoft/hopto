package icons

import (
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// icnsFile writes a fake .icns with the given entries into a temp folder.
func icnsFile(t *testing.T, entries ...[]byte) string {
	t.Helper()

	body := []byte{}
	for _, entry := range entries {
		body = append(body, entry...)
	}

	file := append([]byte("icns"), 0, 0, 0, 0)
	binary.BigEndian.PutUint32(file[4:], uint32(8+len(body)))
	file = append(file, body...)

	path := filepath.Join(t.TempDir(), "x.icns")
	if err := os.WriteFile(path, file, 0o644); err != nil {
		t.Fatal(err)
	}

	return path
}

// entry builds one icns entry of the given type around a payload.
func entry(kind string, payload []byte) []byte {
	out := append([]byte(kind), 0, 0, 0, 0)
	binary.BigEndian.PutUint32(out[4:], uint32(8+len(payload)))

	return append(out, payload...)
}

// TestPNGPrefers256 checks the 256 px PNG (ic08) wins over 128 px.
func TestPNGPrefers256(t *testing.T) {
	png, ok := PNG("testdata/edge-app.icns")
	if !ok {
		t.Fatal("no PNG found in the test icns")
	}

	// The test icns holds ic07 (4098 bytes of PNG) and ic08 (9399).
	if len(png) != 9399 {
		t.Errorf("png length = %d, want the ic08 entry (9399)", len(png))
	}
}

// Files that are not a usable icns: garbage, truncated, only JPEG 2000,
// no PNG at all, or shorter than a header pair.
func TestPNGRejects(t *testing.T) {
	pngMagic := []byte("\x89PNG\r\n\x1a\n")
	jp2Magic := []byte("\x00\x00\x00\x0cjP  \r\n\x87\n")

	cases := map[string]string{
		"garbage":           filepath.Join(t.TempDir(), "missing.icns"),
		"truncated entry":   icnsFile(t, []byte("ic08\x00\x00\x10\x00"), pngMagic),
		"length past file":  icnsFile(t, entry("ic08", pngMagic)[:10]),
		"jpeg 2000 in ic08": icnsFile(t, entry("ic08", jp2Magic)),
		"no png entries":    icnsFile(t, entry("it32", []byte("raw"))),
		"shorter than 16":   icnsFile(t),
	}

	for name, path := range cases {
		if _, ok := PNG(path); ok {
			t.Errorf("%s: accepted", name)
		}
	}
}

// A PNG in a lesser slot is still returned when the best ones are absent.
func TestPNGFallsBackToSmallerSizes(t *testing.T) {
	pngMagic := []byte("\x89PNG\r\n\x1a\nrest")
	path := icnsFile(t, entry("ic12", pngMagic))

	png, ok := PNG(path)
	if !ok || string(png) != string(pngMagic) {
		t.Fatalf("got %q, %v", png, ok)
	}
}
