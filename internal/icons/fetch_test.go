package icons

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// solidPNG is a valid PNG of the given side.
func solidPNG(t *testing.T, side int) []byte {
	t.Helper()

	img := image.NewNRGBA(image.Rect(0, 0, side, side))
	for y := range side {
		for x := range side {
			img.Set(x, y, color.NRGBA{R: 10, G: 120, B: 200, A: 255})
		}
	}

	var out bytes.Buffer
	if err := png.Encode(&out, img); err != nil {
		t.Fatal(err)
	}

	return out.Bytes()
}

// iconServer serves the candidates the tests point at: a good icon, a bad
// one of each kind, and a page whose head links to them.
func iconServer(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /good.png", func(
		w http.ResponseWriter, r *http.Request,
	) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(solidPNG(t, 64))
	})
	mux.HandleFunc("GET /sh/name.png", func(
		w http.ResponseWriter, r *http.Request,
	) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(solidPNG(t, 256))
	})
	mux.HandleFunc("GET /html.png", func(
		w http.ResponseWriter, r *http.Request,
	) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write([]byte("<html>not an icon</html>"))
	})
	mux.HandleFunc("GET /huge.png", func(
		w http.ResponseWriter, r *http.Request,
	) {
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(make([]byte, maxIconBytes+1))
	})
	mux.HandleFunc("GET /page", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><head>
<link rel="icon" sizes="512x512" href="/huge.png">
<link rel="icon" sizes="256x256" href="/html.png">
<link rel="icon" type="image/svg+xml" href="/v.svg">
<link rel="icon" sizes="64x64" href="/good.png">
</head></html>`))
	})

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	return server
}

// testFetcher reaches loopback and http, as a user with
// allow_private_icon_hosts would.
func testFetcher(server *httptest.Server, services ...string) *Fetcher {
	fetcher := NewFetcher(true, services)
	fetcher.selfhstBase = server.URL + "/sh/"

	return fetcher
}

// The order the spec fixes: hint, then the site's own icons, then each
// third-party service in the order the settings list them; http dropped
// unless allowed; duplicates removed.
func TestCandidatesOrderAndFilter(t *testing.T) {
	icons := []string{
		"https://a.test/apple.png",
		"http://a.test/plain.png",
		"https://a.test/apple.png",
	}

	got := Candidates(
		"sh:github", icons, "a.test",
		[]string{"site", "google", "duckduckgo"},
		"https://cdn.test/png/", false,
	)

	want := []string{
		"https://cdn.test/png/github.png",
		"https://a.test/apple.png",
		"https://www.google.com/s2/favicons?domain=a.test&sz=128",
		"https://icons.duckduckgo.com/ip3/a.test.ico",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf(
			"got\n%s\nwant\n%s",
			strings.Join(got, "\n"), strings.Join(want, "\n"),
		)
	}

	// With http allowed the plain icon stays; without "site" the page's
	// icons are not even listed; an https hint is a candidate as it is.
	withHTTP := Candidates(
		"https://b.test/i.png", icons, "a.test", []string{"site"},
		"https://cdn.test/png/", true,
	)
	if len(withHTTP) != 3 || withHTTP[0] != "https://b.test/i.png" ||
		withHTTP[2] != "http://a.test/plain.png" {
		t.Errorf("with http: %v", withHTTP)
	}

	got = Candidates("", icons, "a.test", []string{"google"}, "", false)
	if len(got) != 1 {
		t.Errorf("without site: %v", got)
	}
}

// A hint that names a selfh.st icon is fetched from the pinned CDN path.
func TestFetchUsesTheHint(t *testing.T) {
	server := iconServer(t)

	data, err := testFetcher(server).Fetch(
		context.Background(), "sh:name", server.URL+"/nowhere",
	)
	if err != nil {
		t.Fatal(err)
	}

	config, err := png.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width != LinkSide {
		t.Errorf(
			"got %dx%d (%v), want %d",
			config.Width, config.Height, err, LinkSide,
		)
	}
}

// Bad candidates are skipped in order until one decodes: the page lists a
// too-large icon first, HTML as PNG second, an SVG (dropped at parse) and
// finally the good one.
func TestFetchSkipsBadCandidates(t *testing.T) {
	server := iconServer(t)

	data, err := testFetcher(server, "site").Fetch(
		context.Background(), server.URL+"/html.png", server.URL+"/page",
	)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.HasPrefix(data, []byte("\x89PNG")) {
		t.Error("output is not a PNG")
	}
}

// Nothing usable anywhere is a clean ErrNoIcon.
func TestFetchReportsNoIcon(t *testing.T) {
	server := iconServer(t)

	_, err := testFetcher(server).Fetch(
		context.Background(), server.URL+"/html.png", server.URL+"/404",
	)
	if !errors.Is(err, ErrNoIcon) {
		t.Errorf("err = %v, want ErrNoIcon", err)
	}
}

// The default fetcher never reaches loopback: the whole fetch fails
// without a connection.
func TestFetchRefusesPrivateHostsByDefault(t *testing.T) {
	server := iconServer(t)

	_, err := NewFetcher(false, []string{"site"}).Fetch(
		context.Background(), server.URL+"/good.png", server.URL+"/page",
	)
	if !errors.Is(err, ErrNoIcon) {
		t.Errorf("err = %v, want ErrNoIcon", err)
	}
}
