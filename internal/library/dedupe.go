package library

import (
	"net/url"
	"strings"
)

// NormalizeURL reduces a URL to what identifies the page, so two ways of
// typing the same address compare equal: lowercase scheme and host, no
// "www.", no default port, no fragment, no trailing slash. It never
// changes the scheme: http and https are different links.
func NormalizeURL(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return strings.TrimSpace(raw)
	}

	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")

	port := parsed.Port()
	isDefaultPort := (parsed.Scheme == "https" && port == "443") || (parsed.Scheme == "http" && port == "80")
	if port != "" && !isDefaultPort {
		host += ":" + port
	}

	path := strings.TrimRight(parsed.EscapedPath(), "/")
	query := ""
	if parsed.RawQuery != "" {
		query = "?" + parsed.RawQuery
	}

	return strings.ToLower(parsed.Scheme) + "://" + host + path + query
}

// FindDuplicateLink returns the link that points at the same page as raw.
func FindDuplicateLink(links []Link, raw string) (Link, bool) {
	wanted := NormalizeURL(raw)

	for _, link := range links {
		if NormalizeURL(link.URL) == wanted {
			return link, true
		}
	}

	return Link{}, false
}

// LinksOnSameHost returns the links that share the host of raw; the editor
// shows them as a hint, not as a block.
func LinksOnSameHost(links []Link, raw string) []Link {
	wanted := hostOnly(raw)
	neighbours := []Link{}

	for _, link := range links {
		if hostOnly(link.URL) == wanted {
			neighbours = append(neighbours, link)
		}
	}

	return neighbours
}

// hostOnly is the normalised host of a URL, "" when it has none.
func hostOnly(raw string) string {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}

	return strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
}
