package main

import (
	"strings"
	"testing"
)

// TestEmbeddedLinksAreValid is the guard for hand edits to links.json: it
// fails the build if a link has no id, repeats one, points at an unknown
// category or is not a web URL. Plan 2 replaces links.json with a different
// data source, so this currently accepts an empty catalog.
func TestEmbeddedLinksAreValid(t *testing.T) {
	catalog, err := loadLinks(linksJSON)
	if err != nil {
		t.Fatal(err)
	}

	for _, link := range catalog.Links {
		if link.Host == "" {
			t.Errorf("link %q has no host after loading", link.ID)
		}
	}
}

// TestLoadLinksRejectsBadCatalogs covers each validation rule with the
// smallest catalog that breaks it.
func TestLoadLinksRejectsBadCatalogs(t *testing.T) {
	cases := []struct {
		name string
		json string
		want string
	}{
		{
			name: "unknown category",
			json: `{"categories":[{"id":"a","name":"A"}],"links":[{"id":"x","name":"X","url":"https://x.test","category":"b"}]}`,
			want: "unknown category",
		},
		{
			name: "duplicate link",
			json: `{"categories":[{"id":"a","name":"A"}],"links":[{"id":"x","name":"X","url":"https://x.test","category":"a"},{"id":"x","name":"Y","url":"https://y.test","category":"a"}]}`,
			want: "duplicate link",
		},
		{
			name: "duplicate category",
			json: `{"categories":[{"id":"a","name":"A"},{"id":"a","name":"B"}],"links":[]}`,
			want: "duplicate category",
		},
		{
			name: "not a web URL",
			json: `{"categories":[{"id":"a","name":"A"}],"links":[{"id":"x","name":"X","url":"file:///etc/passwd","category":"a"}]}`,
			want: "not a web URL",
		},
		{
			name: "missing id",
			json: `{"categories":[{"id":"a","name":"A"}],"links":[{"name":"X","url":"https://x.test","category":"a"}]}`,
			want: "without id",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadLinks([]byte(tc.json))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got error %v, want one containing %q", err, tc.want)
			}
		})
	}
}

// TestLoadLinksFillsHost checks the derived host the page searches on.
func TestLoadLinksFillsHost(t *testing.T) {
	catalog, err := loadLinks([]byte(`{"categories":[{"id":"a","name":"A"}],"links":[{"id":"x","name":"X","url":"https://docs.example.org/path?q=1","category":"a"}]}`))
	if err != nil {
		t.Fatal(err)
	}

	if got := catalog.Links[0].Host; got != "docs.example.org" {
		t.Errorf("host = %q, want docs.example.org", got)
	}
}

// TestFindLink looks a catalog entry up. Since plan 2 removes links.json,
// this tests with an empty catalog and a missing link.
func TestFindLink(t *testing.T) {
	if _, ok := findLink("no-such-link"); ok {
		t.Error("findLink found a link that does not exist")
	}
}
