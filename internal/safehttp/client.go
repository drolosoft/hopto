// Package safehttp is the only way hopto talks to the network. Every fetch
// runs with nobody watching (icons come down in the background, a URL is
// inspected while the user types), so the client refuses what a browser
// would happily do: private and loopback addresses, plain http, endless
// redirects and unbounded bodies.
package safehttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"syscall"
	"time"
)

// ErrPrivateAddress is returned when the host resolves to an address the
// options do not allow: loopback, RFC 1918, link-local, CGNAT (the tailnet)
// or unspecified.
var ErrPrivateAddress = errors.New("safehttp: address is private")

// ErrScheme is returned for a URL that is not https (or http when allowed).
var ErrScheme = errors.New("safehttp: only https is allowed")

// ErrTooLarge is returned when the body passes the caller's cap.
var ErrTooLarge = errors.New("safehttp: body too large")

// userAgent names hopto to the sites it fetches from, as good manners and
// so an operator can tell where a request came from.
const userAgent = "hopto/0.1 (+https://github.com/drolosoft/hopto)"

// defaultRedirects is what a favicon or an apple-touch-icon needs at most:
// one hop to www, one to https, one to a CDN.
const defaultRedirects = 3

// maxHeaderBytes caps the response headers; a page never needs more and a
// hostile server should not be able to fill memory with them.
const maxHeaderBytes = 64 << 10

// cgnat is 100.64.0.0/10, the range Tailscale (and carriers) hand out; Go's
// IsPrivate does not cover it.
var cgnat = netip.MustParsePrefix("100.64.0.0/10")

// Options are the few knobs of a client.
type Options struct {
	// AllowPrivate lets connections reach loopback, private, link-local and
	// CGNAT addresses (self-hosted sites on a LAN or a tailnet).
	AllowPrivate bool

	// AllowHTTP accepts plain http URLs and redirects to them.
	AllowHTTP bool

	// Timeout covers the whole request, dial included.
	Timeout time.Duration

	// MaxRedirects is the number of hops followed; 0 means defaultRedirects.
	MaxRedirects int
}

// IsPrivateAddr reports whether an address must not be reached without the
// opt-in. IPv4-mapped IPv6 addresses are unmapped first, so ::ffff:10.0.0.1
// counts as 10.0.0.1.
func IsPrivateAddr(addr netip.Addr) bool {
	addr = addr.Unmap()

	if addr.IsLoopback() || addr.IsPrivate() || addr.IsUnspecified() {
		return true
	}

	if addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() || addr.IsMulticast() {
		return true
	}

	return cgnat.Contains(addr)
}

// NewClient builds a client with the checks wired in. The environment proxy
// is ignored on purpose: a proxy would connect on our behalf and skip the
// address check.
func NewClient(opts Options) *http.Client {
	if opts.MaxRedirects == 0 {
		opts.MaxRedirects = defaultRedirects
	}

	dialer := &net.Dialer{
		Timeout: opts.Timeout,
		Control: func(network, address string, _ syscall.RawConn) error {
			return checkAddress(address, opts.AllowPrivate)
		},
	}

	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            dialer.DialContext,
		TLSHandshakeTimeout:    opts.Timeout,
		ResponseHeaderTimeout:  opts.Timeout,
		MaxResponseHeaderBytes: maxHeaderBytes,
		DisableKeepAlives:      true,
	}

	return &http.Client{
		Transport: transport,
		Timeout:   opts.Timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= opts.MaxRedirects {
				return fmt.Errorf("safehttp: more than %d redirects", opts.MaxRedirects)
			}

			return checkScheme(req.URL.Scheme, opts.AllowHTTP)
		},
	}
}

// Open sends a GET for rawURL and returns the response when the status is
// 200; the caller reads the body with its own cap and closes it. The
// scheme is checked before dialing; CheckRedirect covers the hops.
func Open(
	ctx context.Context, client *http.Client, rawURL string, accept string,
) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}

	if err := checkScheme(request.URL.Scheme, allowsHTTP(client)); err != nil {
		return nil, err
	}

	request.Header.Set("User-Agent", userAgent)
	request.Header.Set("Accept", accept)

	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	if response.StatusCode != http.StatusOK {
		// Best effort: the body of an error page is of no use.
		_ = response.Body.Close()
		return nil, fmt.Errorf(
			"safehttp: %s answered %d", request.URL.Host, response.StatusCode,
		)
	}

	return response, nil
}

// Get fetches rawURL whole and returns the body and its Content-Type. The
// body is read up to maxBytes and refused past it.
func Get(
	ctx context.Context, client *http.Client, rawURL string, accept string,
	maxBytes int64,
) ([]byte, string, error) {
	response, err := Open(ctx, client, rawURL, accept)
	if err != nil {
		return nil, "", err
	}
	defer func() {
		// Best effort: the body is drained by the read below or abandoned.
		_ = response.Body.Close()
	}()

	// One byte more than the cap tells a body that is too large apart from
	// one that is exactly at it.
	body, err := io.ReadAll(io.LimitReader(response.Body, maxBytes+1))
	if err != nil {
		return nil, "", err
	}

	if int64(len(body)) > maxBytes {
		return nil, "", ErrTooLarge
	}

	return body, response.Header.Get("Content-Type"), nil
}

// checkAddress is the dial-time guard: address is "ip:port" with the
// resolved IP, so a DNS name pointing at the LAN is caught here, not by
// looking at the host name.
func checkAddress(address string, allowPrivate bool) error {
	if allowPrivate {
		return nil
	}

	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return err
	}

	addr, err := netip.ParseAddr(host)
	if err != nil {
		return err
	}

	if IsPrivateAddr(addr) {
		return fmt.Errorf("%w: %s", ErrPrivateAddress, host)
	}

	return nil
}

// checkScheme accepts https, and http only when asked for.
func checkScheme(scheme string, allowHTTP bool) error {
	if scheme == "https" || (scheme == "http" && allowHTTP) {
		return nil
	}

	return fmt.Errorf("%w: got %q", ErrScheme, scheme)
}

// allowsHTTP recovers the option from the client, so Get needs no second
// parameter: a client built without AllowHTTP has a CheckRedirect that
// refuses http, and the first request must follow the same rule.
func allowsHTTP(client *http.Client) bool {
	if client.CheckRedirect == nil {
		return false
	}

	probe, _ := http.NewRequest(http.MethodGet, "http://probe.invalid/", nil)

	return client.CheckRedirect(probe, nil) == nil
}
