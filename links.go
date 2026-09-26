package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
)

// linksJSON is the catalog of "mis links". It is data, not code, so adding a
// link or a category is an edit to links.json and a rebuild; the tests check
// it on every run.
//
//go:embed links.json
var linksJSON []byte

// Category groups apps or links under one filter chip on the page.
type Category struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Link is one entry of "mis links". Icon is only a hint for
// scripts/fetch-link-icons.sh (where to get the icon from); the page looks
// the icon up by ID like it does for apps.
type Link struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Category    string `json:"category"`
	Icon        string `json:"icon,omitempty"`

	// Host is derived from URL when the catalog loads, so the page can show
	// and search the domain without parsing anything.
	Host string `json:"host"`
}

// LinkCatalog is the shape of links.json: the categories in the order they
// show on screen, and the links in the order they show inside each one.
type LinkCatalog struct {
	Categories []Category `json:"categories"`
	Links      []Link     `json:"links"`
}

// linkCatalog is the parsed and validated links.json. A broken file fails
// at start, which is what we want: the tests catch it before a build.
var linkCatalog = mustLoadLinks(linksJSON)

// mustLoadLinks parses the embedded catalog or panics with the reason.
func mustLoadLinks(data []byte) LinkCatalog {
	catalog, err := loadLinks(data)
	if err != nil {
		panic("links.json: " + err.Error())
	}

	return catalog
}

// loadLinks parses a catalog, checks it and fills the derived fields. Every
// link needs a unique id, a category that exists and an http(s) URL; those
// are the mistakes that are easy to make when adding links by hand.
func loadLinks(data []byte) (LinkCatalog, error) {
	var catalog LinkCatalog

	if err := json.Unmarshal(data, &catalog); err != nil {
		return catalog, err
	}

	known := make(map[string]bool, len(catalog.Categories))
	for _, category := range catalog.Categories {
		if category.ID == "" || category.Name == "" {
			return catalog, fmt.Errorf("category without id or name: %+v", category)
		}

		if known[category.ID] {
			return catalog, fmt.Errorf("duplicate category %q", category.ID)
		}

		known[category.ID] = true
	}

	seen := make(map[string]bool, len(catalog.Links))
	for index := range catalog.Links {
		link := &catalog.Links[index]

		if link.ID == "" || link.Name == "" {
			return catalog, fmt.Errorf("link without id or name: %+v", *link)
		}

		if seen[link.ID] {
			return catalog, fmt.Errorf("duplicate link %q", link.ID)
		}

		seen[link.ID] = true

		if !known[link.Category] {
			return catalog, fmt.Errorf("link %q: unknown category %q", link.ID, link.Category)
		}

		host, err := hostOf(link.URL)
		if err != nil {
			return catalog, fmt.Errorf("link %q: %w", link.ID, err)
		}

		link.Host = host
	}

	return catalog, nil
}

// hostOf returns the host of an http or https URL, or an error for anything
// else: the launcher hands links to `open`, and only web URLs belong here.
func hostOf(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", err
	}

	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", fmt.Errorf("not a web URL: %q", raw)
	}

	return parsed.Host, nil
}

// findLink returns the link with the given id, if any.
func findLink(id string) (Link, bool) {
	for _, link := range linkCatalog.Links {
		if link.ID == id {
			return link, true
		}
	}

	return Link{}, false
}
