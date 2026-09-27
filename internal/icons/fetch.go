package icons

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/drolosoft/hopto/internal/inspect"
	"github.com/drolosoft/hopto/internal/safehttp"
)

// ErrNoIcon is returned when no candidate produced a usable image.
var ErrNoIcon = errors.New("icons: no usable icon found")

// selfhstBase is the selfh.st icon collection on jsDelivr, pinned to a
// commit so a renamed or removed icon upstream never changes what a hint
// resolves to. Bump the commit by hand when new icons are wanted.
const selfhstBase = "https://cdn.jsdelivr.net/gh/selfhst/icons@" +
	"589d718a638b7770abae0edd1b60ff36c0dd1d5a/png/"

// FetchTimeout bounds one candidate; a favicon is a few kilobytes and a
// server that takes longer is not going to answer.
const FetchTimeout = 10 * time.Second

// wholeFetchTimeout bounds the whole chain of candidates, so a link with a
// slow site and two slow services still gives up in a reasonable time.
const wholeFetchTimeout = 30 * time.Second

// maxIconBytes is more than any real icon and far less than an image made
// to fill memory.
const maxIconBytes = 512 << 10

// maxCandidates keeps a page that declares dozens of icons from being
// fetched dozens of times.
const maxCandidates = 8

// accept tells the server what hopto can decode.
const iconAccept = "image/png,image/jpeg,image/gif;q=0.9,*/*;q=0.1"

// Service names as they appear in settings.icon_services. DuckDuckGo
// answers with a .ico, which Go's image package cannot decode; the
// service stays in the list anyway, since some hosts serve a PNG under
// that path, and a miss only costs one extra request.
const (
	serviceSite       = "site"
	serviceGoogle     = "google"
	serviceDuckDuckGo = "duckduckgo"
)

// Fetcher downloads the icon of a link. One per App, built from settings.
type Fetcher struct {
	client      *http.Client
	allowHTTP   bool
	services    []string
	selfhstBase string
}

// NewFetcher builds a fetcher. allowPrivate is the user's opt-in for
// self-hosted sites: it opens both private addresses and plain http, since
// those hosts are the ones that speak http.
func NewFetcher(allowPrivate bool, services []string) *Fetcher {
	options := safehttp.Options{
		AllowPrivate: allowPrivate,
		AllowHTTP:    allowPrivate,
		Timeout:      FetchTimeout,
	}

	return &Fetcher{
		client:      safehttp.NewClient(options),
		allowHTTP:   allowPrivate,
		services:    append([]string{}, services...),
		selfhstBase: selfhstBase,
	}
}

// Fetch tries every candidate in order and returns the first one that
// decodes, normalised to LinkSide. Failures of single candidates are not
// errors: the next one is tried; only running out of them is.
func (f *Fetcher) Fetch(
	ctx context.Context, hint, pageURL string,
) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, wholeFetchTimeout)
	defer cancel()

	host := ""
	if parsed, err := url.Parse(pageURL); err == nil {
		host = parsed.Hostname()
	}

	pageIcons := []string{}
	if slices.Contains(f.services, serviceSite) {
		// A page that cannot be read still leaves the hint and the
		// services.
		if page, err := inspect.Fetch(ctx, f.client, pageURL); err == nil {
			pageIcons = page.Icons
		}
	}

	candidates := Candidates(
		hint, pageIcons, host, f.services, f.selfhstBase, f.allowHTTP,
	)

	for _, candidate := range candidates {
		body, _, err := safehttp.Get(
			ctx, f.client, candidate, iconAccept, maxIconBytes,
		)
		if err != nil {
			continue
		}

		icon, err := Normalize(body, LinkSide)
		if err != nil {
			continue
		}

		return icon, nil
	}

	return nil, ErrNoIcon
}

// Candidates lists the URLs to try, best first: the hint, the page's own
// icons (when "site" is a service), then each third-party service in the
// order the user listed them. Plain http survives only when allowed.
func Candidates(
	hint string, pageIcons []string, host string, services []string,
	selfhstBase string, allowHTTP bool,
) []string {
	candidates := []string{}

	switch {
	case strings.HasPrefix(hint, "sh:"):
		name := strings.TrimPrefix(hint, "sh:")
		candidates = append(candidates, selfhstBase+name+".png")
	case hint != "":
		candidates = append(candidates, hint)
	}

	for _, service := range services {
		switch service {
		case serviceSite:
			candidates = append(candidates, pageIcons...)
		case serviceGoogle:
			if host != "" {
				candidates = append(candidates,
					"https://www.google.com/s2/favicons?domain="+
						url.QueryEscape(host)+"&sz=128")
			}
		case serviceDuckDuckGo:
			if host != "" {
				candidates = append(candidates,
					"https://icons.duckduckgo.com/ip3/"+
						url.PathEscape(host)+".ico")
			}
		}
	}

	seen := map[string]bool{}
	kept := []string{}

	for _, candidate := range candidates {
		if seen[candidate] || len(kept) == maxCandidates {
			continue
		}

		isHTTPS := strings.HasPrefix(candidate, "https://")
		isAllowedHTTP := allowHTTP && strings.HasPrefix(candidate, "http://")
		if !isHTTPS && !isAllowedHTTP {
			continue
		}

		seen[candidate] = true
		kept = append(kept, candidate)
	}

	return kept
}
