// Package inspect reads what a web page says about itself (title, site
// name, description, icons) so a link can be added from its URL alone. It
// reads the head only, through safehttp, and never fails the link over a
// page it cannot read: a link to a PDF is still a link.
package inspect

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/net/html/charset"

	"github.com/drolosoft/hopto/internal/safehttp"
)

// Timeout is how long the user waits for a preview while typing a URL.
const Timeout = 5 * time.Second

// maxBodyBytes is read at most; the head of any page fits in far less and
// the rest is not needed.
const maxBodyBytes = 1 << 20

// The library's own limits, applied here so the draft the editor shows is
// already something the store accepts.
const (
	maxNameRunes        = 80
	maxDescriptionRunes = 200
)

// appleTouchWeight ranks any apple-touch-icon above any favicon: it is
// square, big and made to be a tile, which is what the row shows.
const appleTouchWeight = 1000

// accept asks for HTML and tolerates anything, so a server that insists on
// another type still answers with a 200 we can ignore.
const accept = "text/html;q=1.0, application/xhtml+xml;q=0.9, */*;q=0.1"

// Page is what was learnt about a URL. Icons are absolute, best first.
type Page struct {
	URL         string
	Host        string
	Title       string
	SiteName    string
	Description string
	Icons       []string
}

// iconCandidate is a <link rel=icon> with the rank its rel and sizes give.
type iconCandidate struct {
	href   string
	weight int
}

// SuggestedName is what the editor puts in the name field: the site name
// when the page declares one, else the title, else the host.
func (p Page) SuggestedName() string {
	if p.SiteName != "" {
		return p.SiteName
	}

	if p.Title != "" {
		return p.Title
	}

	return p.Host
}

// Fetch downloads rawURL and parses its head. The returned page always
// carries URL and Host; the error is only for a request that failed.
func Fetch(
	ctx context.Context, client *http.Client, rawURL string,
) (Page, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return Page{URL: rawURL, Icons: []string{}}, err
	}

	page := Page{URL: rawURL, Host: parsed.Hostname(), Icons: []string{}}

	ctx, cancel := context.WithTimeout(ctx, Timeout)
	defer cancel()

	response, err := safehttp.Open(ctx, client, rawURL, accept)
	if err != nil {
		return page, err
	}
	defer func() {
		// Best effort: the page was read or is being abandoned.
		_ = response.Body.Close()
	}()

	contentType := response.Header.Get("Content-Type")
	if !strings.Contains(contentType, "html") {
		return page, nil
	}

	// Truncating is fine here: the head comes first and a page longer than
	// the cap is still a page.
	limited := io.LimitReader(response.Body, maxBodyBytes)

	reader, err := charset.NewReader(limited, contentType)
	if err != nil {
		reader = limited
	}

	// Icons resolve against where the page ended up after redirects; the
	// URL the user typed stays as the link.
	parsed = response.Request.URL
	parsedPage := Parse(reader, parsed)
	parsedPage.URL = rawURL
	parsedPage.Host = page.Host

	return parsedPage, nil
}

// Parse reads the head of an HTML document: the title, the Open Graph
// site name, the description and every icon link, resolved against base.
// It stops at the body, so a huge page costs nothing past its head.
func Parse(body io.Reader, base *url.URL) Page {
	page := Page{URL: base.String(), Host: base.Hostname(), Icons: []string{}}
	tokenizer := html.NewTokenizer(body)

	var candidates []iconCandidate
	var title, ogTitle strings.Builder
	inTitle := false

	for {
		switch tokenizer.Next() {
		case html.ErrorToken:
			return finish(page, title.String(), ogTitle.String(), candidates, base)

		case html.StartTagToken, html.SelfClosingTagToken:
			token := tokenizer.Token()

			switch token.DataAtom {
			case atom.Body:
				return finish(page, title.String(), ogTitle.String(), candidates, base)
			case atom.Title:
				inTitle = true
			case atom.Meta:
				readMeta(&page, &ogTitle, token)
			case atom.Link:
				if candidate, ok := iconLink(token, base); ok {
					candidates = append(candidates, candidate)
				}
			}

		case html.EndTagToken:
			token := tokenizer.Token()
			if token.DataAtom == atom.Title {
				inTitle = false
			}

			if token.DataAtom == atom.Head {
				return finish(page, title.String(), ogTitle.String(), candidates, base)
			}

		case html.TextToken:
			if inTitle {
				title.Write(tokenizer.Text())
			}
		}
	}
}

// readMeta picks the meta tags the editor uses.
func readMeta(page *Page, ogTitle *strings.Builder, token html.Token) {
	name := strings.ToLower(attribute(token, "property"))
	if name == "" {
		name = strings.ToLower(attribute(token, "name"))
	}

	content := attribute(token, "content")

	switch name {
	case "og:site_name":
		page.SiteName = content
	case "og:title":
		ogTitle.Reset()
		ogTitle.WriteString(content)
	case "description", "og:description":
		if page.Description == "" {
			page.Description = content
		}
	}
}

// iconLink turns a <link> into a candidate when its rel names an icon we
// can decode: .ico and SVG are skipped because Go has no decoder for them
// and fetching them would be a wasted round trip.
func iconLink(token html.Token, base *url.URL) (iconCandidate, bool) {
	rel := strings.Fields(strings.ToLower(attribute(token, "rel")))
	isIcon, isApple := false, false

	for _, word := range rel {
		switch word {
		case "icon":
			isIcon = true
		case "apple-touch-icon", "apple-touch-icon-precomposed":
			isIcon, isApple = true, true
		}
	}

	href := attribute(token, "href")
	if !isIcon || href == "" {
		return iconCandidate{}, false
	}

	kind := strings.ToLower(attribute(token, "type"))
	lower := strings.ToLower(href)
	noDecoder := strings.HasSuffix(lower, ".svg") ||
		strings.HasSuffix(lower, ".ico")
	if strings.Contains(kind, "svg") || noDecoder {
		return iconCandidate{}, false
	}

	resolved, err := base.Parse(href)
	if err != nil ||
		(resolved.Scheme != "https" && resolved.Scheme != "http") {
		return iconCandidate{}, false
	}

	weight := sizeOf(attribute(token, "sizes"))
	if isApple {
		weight += appleTouchWeight
	}

	return iconCandidate{href: resolved.String(), weight: weight}, true
}

// sizeOf reads the width of a sizes attribute ("192x192", "any" → 0).
func sizeOf(sizes string) int {
	first := strings.Fields(strings.ToLower(sizes))
	if len(first) == 0 {
		return 0
	}

	width, _, _ := strings.Cut(first[0], "x")
	value, err := strconv.Atoi(width)
	if err != nil {
		return 0
	}

	return value
}

// finish orders the candidates, adds the conventional apple-touch-icon.png
// at the root as a last guess, and tidies the texts.
func finish(
	page Page, title, ogTitle string, candidates []iconCandidate,
	base *url.URL,
) Page {
	sort.SliceStable(candidates, func(left, right int) bool {
		return candidates[left].weight > candidates[right].weight
	})

	seen := map[string]bool{}
	for _, candidate := range candidates {
		if !seen[candidate.href] {
			seen[candidate.href] = true
			page.Icons = append(page.Icons, candidate.href)
		}
	}

	guess := base.Scheme + "://" + base.Host + "/apple-touch-icon.png"
	if !seen[guess] {
		page.Icons = append(page.Icons, guess)
	}

	if title == "" {
		title = ogTitle
	}

	page.Title = tidy(title, maxNameRunes)
	page.SiteName = tidy(page.SiteName, maxNameRunes)
	page.Description = tidy(page.Description, maxDescriptionRunes)

	return page
}

// tidy collapses whitespace, drops control and invisible format
// characters, and cuts to a number of runes, so the text fits the
// library's rules as it is.
func tidy(text string, maxRunes int) string {
	clean := strings.Map(func(r rune) rune {
		switch {
		case unicode.IsControl(r):
			return ' '
		case unicode.Is(unicode.Cf, r):
			// A format character (zero-width space, bidi marks, the byte
			// order mark) never means a word break the way a control
			// character does, so it is dropped outright rather than
			// turned into a space that would split one word into two.
			return -1
		default:
			return r
		}
	}, text)

	words := strings.Fields(clean)
	runes := []rune(strings.Join(words, " "))

	if len(runes) > maxRunes {
		runes = runes[:maxRunes]
	}

	return string(runes)
}

// attribute returns the value of an attribute of a token, "" when absent.
func attribute(token html.Token, name string) string {
	for _, attr := range token.Attr {
		if attr.Key == name {
			return strings.TrimSpace(attr.Val)
		}
	}

	return ""
}
