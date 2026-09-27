package inspect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/drolosoft/hopto/internal/safehttp"
)

// A head with everything hopto reads, including the noise it must skip: a
// favicon.ico (no decoder), an SVG icon, a relative href, a title full of
// whitespace and a second title inside the body.
const fixture = `<!DOCTYPE html>
<html><head>
<meta charset="utf-8">
<title>
   Example   Site  ·  Home
</title>
<meta property="og:site_name" content="Example">
<meta name="description" content="  A site about examples.  ">
<link rel="icon" href="/favicon.ico">
<link rel="icon" type="image/png" sizes="32x32" href="/i32.png">
<link rel="icon" type="image/png" sizes="192x192" href="i192.png">
<link rel="apple-touch-icon" href="/apple.png">
<link rel="icon" type="image/svg+xml" href="/v.svg">
</head><body><title>Not this one</title></body></html>`

// testClient reaches httptest's loopback server.
func testClient() *http.Client {
	return safehttp.NewClient(safehttp.Options{
		AllowPrivate: true,
		AllowHTTP:    true,
		Timeout:      time.Second,
	})
}

// newHTMLServer starts a test server that answers every request with the
// given handler, so each test only states what that one response looks
// like.
func newHTMLServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

// TestParseReadsTheHead takes the title, the site name, the description
// and the icons from the head of a page.
func TestParseReadsTheHead(t *testing.T) {
	base, _ := url.Parse("https://example.test/docs/")

	page := Parse(strings.NewReader(fixture), base)

	if page.Title != "Example Site · Home" {
		t.Errorf("title = %q", page.Title)
	}

	wantDescription := "A site about examples."
	if page.SiteName != "Example" || page.Description != wantDescription {
		t.Errorf("site %q description %q", page.SiteName, page.Description)
	}

	want := []string{
		"https://example.test/apple.png",
		"https://example.test/docs/i192.png",
		"https://example.test/i32.png",
		"https://example.test/apple-touch-icon.png",
	}
	if strings.Join(page.Icons, " ") != strings.Join(want, " ") {
		t.Errorf("icons = %v, want %v", page.Icons, want)
	}
}

// Name precedence: site name, then title, then host.
func TestSuggestedName(t *testing.T) {
	cases := []struct {
		page Page
		want string
	}{
		{
			Page{SiteName: "Example", Title: "Example · Home", Host: "example.test"},
			"Example",
		},
		{Page{Title: "Example · Home", Host: "example.test"}, "Example · Home"},
		{Page{Host: "example.test"}, "example.test"},
	}

	for _, tc := range cases {
		if got := tc.page.SuggestedName(); got != tc.want {
			t.Errorf("%+v: got %q, want %q", tc.page, got, tc.want)
		}
	}
}

// Long titles and descriptions are cut to what the library accepts.
func TestParseCutsLongText(t *testing.T) {
	long := strings.Repeat("é", 300)
	html := "<html><head><title>" + long + "</title>" +
		"<meta name=description content=\"" + long + "\"></head></html>"
	base, _ := url.Parse("https://example.test/")

	page := Parse(strings.NewReader(html), base)

	if got := len([]rune(page.Title)); got != 80 {
		t.Errorf("title has %d runes, want 80", got)
	}

	if got := len([]rune(page.Description)); got != 200 {
		t.Errorf("description has %d runes, want 200", got)
	}
}

// A zero-width space is invisible but not a control character; tidy must
// still drop it, so a page cannot smuggle a title that looks identical to
// another one but compares as different, the same reasoning library.HostOf
// already applies to URLs.
func TestParseDropsFormatCharacters(t *testing.T) {
	html := "<html><head><title>Ex\u200bample</title></head></html>"
	base, _ := url.Parse("https://example.test/")

	page := Parse(strings.NewReader(html), base)

	if page.Title != "Example" {
		t.Errorf("title = %q, want %q", page.Title, "Example")
	}
}

// TestFetchParsesAnHTMLPage fetches a page through the safe client and
// parses it.
func TestFetchParsesAnHTMLPage(t *testing.T) {
	server := newHTMLServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(fixture))
	})
	defer server.Close()

	page, err := Fetch(context.Background(), testClient(), server.URL+"/docs/")
	if err != nil {
		t.Fatal(err)
	}

	if page.SiteName != "Example" || page.Host != "127.0.0.1" ||
		len(page.Icons) != 4 {
		t.Errorf("page = %+v", page)
	}
}

// A Latin-1 page comes out as UTF-8: the charset of the Content-Type is
// honoured.
func TestFetchHonoursTheCharset(t *testing.T) {
	server := newHTMLServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=iso-8859-1")
		_, _ = w.Write([]byte("<html><head><title>Caf\xe9</title></head></html>"))
	})
	defer server.Close()

	page, err := Fetch(context.Background(), testClient(), server.URL)
	if err != nil {
		t.Fatal(err)
	}

	if page.Title != "Café" {
		t.Errorf("title = %q", page.Title)
	}
}

// A link to a PDF or an image is still a fine link: no error, just the host.
func TestFetchIgnoresNonHTML(t *testing.T) {
	server := newHTMLServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write([]byte("%PDF-1.4"))
	})
	defer server.Close()

	page, err := Fetch(context.Background(), testClient(), server.URL+"/x.pdf")
	if err != nil {
		t.Fatal(err)
	}

	if page.Title != "" || page.Host != "127.0.0.1" || page.Icons == nil {
		t.Errorf("page = %+v", page)
	}
}

// TestFetchReportsErrors reports a non-2xx status as an error instead of
// parsing the body.
func TestFetchReportsErrors(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	_, err := Fetch(context.Background(), testClient(), server.URL)
	if err == nil {
		t.Fatal("a 404 gave no error")
	}
}

// The head of a huge page is still read: the cap truncates, it does not
// refuse.
func TestFetchReadsTheHeadOfAHugePage(t *testing.T) {
	server := newHTMLServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><head><title>Big</title></head><body>"))
		_, _ = w.Write([]byte(strings.Repeat("x", 2<<20)))
	})
	defer server.Close()

	page, err := Fetch(context.Background(), testClient(), server.URL)
	if err != nil {
		t.Fatal(err)
	}

	if page.Title != "Big" {
		t.Errorf("title = %q", page.Title)
	}
}
