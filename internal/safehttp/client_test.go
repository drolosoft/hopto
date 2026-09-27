package safehttp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"strings"
	"testing"
	"time"
)

// The addresses a fetch must never reach, and a few it may.
func TestIsPrivateAddr(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1":       true,
		"10.1.2.3":        true,
		"192.168.1.10":    true,
		"172.16.0.1":      true,
		"100.64.0.1":      true,
		"100.127.255.254": true,
		"169.254.1.1":     true,
		"0.0.0.0":         true,
		"224.0.0.1":       true,
		"::1":             true,
		"fc00::1":         true,
		"fe80::1":         true,
		"::ffff:10.0.0.1": true,
		"::":              true,
		"1.1.1.1":         false,
		"100.128.0.1":     false,
		"2606:4700::1111": false,
	}

	for text, want := range cases {
		if got := IsPrivateAddr(netip.MustParseAddr(text)); got != want {
			t.Errorf("IsPrivateAddr(%s) = %v, want %v", text, got, want)
		}
	}
}

// httptest listens on 127.0.0.1, which is exactly what the default client
// must refuse: the request never reaches the handler.
func TestGetRefusesPrivateAddresses(t *testing.T) {
	served := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served = true
	}))
	defer server.Close()

	client := NewClient(Options{AllowHTTP: true, Timeout: time.Second})

	_, _, err := Get(context.Background(), client, server.URL, "*/*", 1024)
	if !errors.Is(err, ErrPrivateAddress) {
		t.Fatalf("err = %v, want ErrPrivateAddress", err)
	}

	if served {
		t.Fatal("the handler ran: the connection was made")
	}
}

// With the opt-in the same server answers.
func TestGetAllowsPrivateWhenAsked(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("hello"))
	}))
	defer server.Close()

	client := NewClient(Options{AllowPrivate: true, AllowHTTP: true, Timeout: time.Second})

	body, contentType, err := Get(context.Background(), client, server.URL, "*/*", 1024)
	if err != nil {
		t.Fatal(err)
	}

	if string(body) != "hello" || contentType != "text/plain" {
		t.Errorf("body %q type %q", body, contentType)
	}
}

// Plain http is refused before any connection unless allowed.
func TestGetRefusesHTTPByDefault(t *testing.T) {
	client := NewClient(Options{AllowPrivate: true, Timeout: time.Second})

	_, _, err := Get(context.Background(), client, "http://example.com/icon.png", "*/*", 1024)
	if !errors.Is(err, ErrScheme) {
		t.Fatalf("err = %v, want ErrScheme", err)
	}
}

// A body past the cap is an error, not a truncated file on disk.
func TestGetRefusesOversizedBodies(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(make([]byte, 2048))
	}))
	defer server.Close()

	client := NewClient(Options{AllowPrivate: true, AllowHTTP: true, Timeout: time.Second})

	_, _, err := Get(context.Background(), client, server.URL, "*/*", 1024)
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("err = %v, want ErrTooLarge", err)
	}
}

// A redirect loop stops after MaxRedirects hops.
func TestGetStopsRedirectLoops(t *testing.T) {
	hops := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hops++
		http.Redirect(w, r, "/again", http.StatusFound)
	}))
	defer server.Close()

	client := NewClient(Options{AllowPrivate: true, AllowHTTP: true, Timeout: time.Second, MaxRedirects: 3})

	_, _, err := Get(context.Background(), client, server.URL, "*/*", 1024)
	if err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("err = %v, want a redirect error", err)
	}

	if hops > 4 {
		t.Errorf("%d hops, want at most 4", hops)
	}
}

// Anything but 200 is an error: a 404 favicon page is not an icon.
func TestGetRefusesNon200(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	defer server.Close()

	client := NewClient(Options{AllowPrivate: true, AllowHTTP: true, Timeout: time.Second})

	if _, _, err := Get(context.Background(), client, server.URL, "*/*", 1024); err == nil {
		t.Fatal("a 404 was accepted")
	}
}

// The caller's context cuts a slow server.
func TestGetHonoursTheContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	defer server.Close()

	client := NewClient(Options{AllowPrivate: true, AllowHTTP: true, Timeout: 5 * time.Second})
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	started := time.Now()
	if _, _, err := Get(ctx, client, server.URL, "*/*", 1024); err == nil {
		t.Fatal("expected a timeout")
	}

	if time.Since(started) > time.Second {
		t.Error("the context deadline was not honoured")
	}
}
