package api

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// HTTPTimeoutEnv names the environment variable that overrides how long the
// client waits for a server's response headers. The value is a Go duration
// ("90s", "5m"); "0" disables the timeout.
const HTTPTimeoutEnv = "REDMINE_HTTP_TIMEOUT"

// Transport timeouts. There is deliberately no overall http.Client.Timeout:
// it would also cap the time spent streaming a response body, which breaks
// large attachment downloads on slow links. Instead each phase that can hang
// on an unresponsive server is bounded.
const (
	dialTimeout                  = 30 * time.Second
	tlsHandshakeTimeout          = 15 * time.Second
	defaultResponseHeaderTimeout = 120 * time.Second
	idleConnTimeout              = 90 * time.Second
)

// maxRedirects matches net/http's default redirect limit.
const maxRedirects = 10

// responseHeaderTimeout returns the configured response header timeout,
// honouring HTTPTimeoutEnv.
func responseHeaderTimeout() (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(HTTPTimeoutEnv))
	if raw == "" {
		return defaultResponseHeaderTimeout, nil
	}
	if raw == "0" {
		return 0, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("invalid %s %q: want a duration such as 90s or 5m, or 0 to disable", HTTPTimeoutEnv, raw)
	}
	return d, nil
}

// newBaseTransport returns a clone of http.DefaultTransport (keeping proxy
// support from the environment) with bounded dial, TLS handshake, response
// header and idle connection timeouts.
func newBaseTransport(headerTimeout time.Duration) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = (&net.Dialer{Timeout: dialTimeout, KeepAlive: 30 * time.Second}).DialContext
	t.TLSHandshakeTimeout = tlsHandshakeTimeout
	t.ResponseHeaderTimeout = headerTimeout
	t.IdleConnTimeout = idleConnTimeout
	return t
}

// origin is a URL's scheme and normalized host:port. Two URLs with the same
// origin are treated as the same server for credential and redirect checks.
type origin struct {
	scheme string
	host   string
}

// originOf returns u's origin: the scheme and hostname are lowercased and an
// omitted port is replaced with the scheme's default, so
// "HTTPS://Redmine.example.com" and "https://redmine.example.com:443" match.
func originOf(u *url.URL) origin {
	scheme := strings.ToLower(u.Scheme)
	port := u.Port()
	if port == "" {
		switch scheme {
		case "https":
			port = "443"
		case "http":
			port = "80"
		}
	}
	return origin{scheme: scheme, host: net.JoinHostPort(strings.ToLower(u.Hostname()), port)}
}

// IsCleartextServer reports whether server is a plain http:// URL on a host
// other than loopback, i.e. whether the API key or password would cross the
// network unencrypted. Loopback servers (localhost, 127.0.0.0/8, ::1) are
// exempt because the traffic never leaves the machine.
func IsCleartextServer(server string) bool {
	u, err := url.Parse(strings.TrimSpace(server))
	if err != nil || !strings.EqualFold(u.Scheme, "http") {
		return false
	}
	return !isLoopbackHost(u.Hostname())
}

// CleartextServerWarning returns the security warning shown for a plain
// http:// server (see IsCleartextServer).
func CleartextServerWarning(server string) string {
	return fmt.Sprintf("server %s uses plain http://, so the API key or password is sent in cleartext; use an https:// URL", debug.ScrubURL(server))
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// checkRedirect is the redirect policy for API requests. It keeps net/http's
// limit of 10 redirects and refuses any redirect that downgrades https to
// http, so the request (and anything the server put in the URL) is never
// resent in cleartext. Credentials are handled separately by authTransport,
// which only attaches them to the configured origin.
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= maxRedirects {
		return fmt.Errorf("stopped after %d redirects", maxRedirects)
	}
	prev := via[len(via)-1]
	if strings.EqualFold(prev.URL.Scheme, "https") && !strings.EqualFold(req.URL.Scheme, "https") {
		return errors.New("refusing redirect from https to insecure http (the request would be resent in cleartext)")
	}
	return nil
}

// downloadRedirectPolicy returns the redirect policy for attachment
// downloads. On top of checkRedirect (redirect cap, no https->http
// downgrade) it only follows redirects that
//   - stay on the configured server's host (any port; credentials are still
//     only attached to the exact configured origin), or
//   - go to another https origin that is not a non-public address (see
//     isBlockedDownloadIP). This keeps installations that offload
//     attachments to object storage (e.g. S3 presigned URLs) working without
//     turning the CLI into a proxy for internal services.
//
// Literal IPs are checked here; host names are re-checked against the IP
// actually dialed by downloadDialer, which defeats DNS rebinding. (net/http
// reports the refused target URL alongside these errors.)
func downloadRedirectPolicy(server *url.URL) func(*http.Request, []*http.Request) error {
	serverHost := server.Hostname()
	return func(req *http.Request, via []*http.Request) error {
		if err := checkRedirect(req, via); err != nil {
			return err
		}
		if strings.EqualFold(req.URL.Hostname(), serverHost) {
			return nil
		}
		if !strings.EqualFold(req.URL.Scheme, "https") {
			return errors.New("refusing attachment redirect: only https redirects away from the configured server are followed")
		}
		if ip := net.ParseIP(req.URL.Hostname()); ip != nil && isBlockedDownloadIP(ip) {
			return fmt.Errorf("refusing attachment redirect: %s is a local, private or otherwise non-public address", ip)
		}
		return nil
	}
}

// blockedDownloadPrefixes are non-public IPv4 ranges that the net/netip
// predicates in isBlockedDownloadAddr do not cover.
var blockedDownloadPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),     // "this network" (RFC 1122)
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT shared space (RFC 6598)
	netip.MustParsePrefix("198.18.0.0/15"), // benchmarking (RFC 2544)
}

// IPv6 prefixes that embed an IPv4 address a gateway may route to.
var (
	nat64Prefix  = netip.MustParsePrefix("64:ff9b::/96") // NAT64 (RFC 6052), IPv4 in the last 4 bytes
	sixToFourNet = netip.MustParsePrefix("2002::/16")    // 6to4 (RFC 3056), IPv4 in bytes 2-5
)

// isBlockedDownloadIP reports whether ip is an address an attachment redirect
// must not reach: loopback, private (RFC 1918, RFC 4193), link-local
// (169.254.0.0/16 including cloud metadata endpoints, fe80::/10), unspecified,
// multicast, 0.0.0.0/8, carrier-grade NAT (100.64.0.0/10) or benchmarking
// (198.18.0.0/15) space. IPv6 addresses that embed an IPv4 address
// (IPv4-mapped ::ffff:0:0/96, NAT64 64:ff9b::/96, 6to4 2002::/16) are judged
// by the embedded address. An unparseable ip is blocked.
func isBlockedDownloadIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	return isBlockedDownloadAddr(addr)
}

func isBlockedDownloadAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if addr.Is6() {
		b := addr.As16()
		switch {
		case nat64Prefix.Contains(addr):
			return isBlockedDownloadAddr(netip.AddrFrom4([4]byte(b[12:16])))
		case sixToFourNet.Contains(addr):
			return isBlockedDownloadAddr(netip.AddrFrom4([4]byte(b[2:6])))
		}
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() || addr.IsInterfaceLocalMulticast() ||
		addr.IsMulticast() || addr.IsUnspecified() {
		return true
	}
	for _, p := range blockedDownloadPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// errBlockedAddress is returned when a download connection would reach an
// address rejected by isBlockedDownloadIP.
var errBlockedAddress = errors.New("refusing to connect: attachment redirect resolves to a local, private or otherwise non-public address")

// downloadDialer dials connections for attachment downloads. Connections to
// the configured server's host, and to any proxy the transport chose, are
// dialed normally (a local or intranet Redmine is expected to live on a
// private address). Every other connection is checked at connect time: the
// resolved IP must pass blocked == false, so a host name that re-resolves to
// an internal address after the redirect check is still refused.
type downloadDialer struct {
	dialer      *net.Dialer
	trustedHost string
	blocked     func(net.IP) bool

	mu         sync.Mutex
	proxyHosts map[string]bool
}

func newDownloadDialer(trustedHost string) *downloadDialer {
	return &downloadDialer{
		dialer:      &net.Dialer{Timeout: dialTimeout, KeepAlive: 30 * time.Second},
		trustedHost: strings.ToLower(trustedHost),
		blocked:     isBlockedDownloadIP,
		proxyHosts:  map[string]bool{},
	}
}

// wrapProxy records the hosts of proxies returned by proxy so DialContext
// lets the transport reach them.
func (d *downloadDialer) wrapProxy(proxy func(*http.Request) (*url.URL, error)) func(*http.Request) (*url.URL, error) {
	if proxy == nil {
		return nil
	}
	return func(req *http.Request) (*url.URL, error) {
		u, err := proxy(req)
		if u != nil {
			d.mu.Lock()
			d.proxyHosts[strings.ToLower(u.Hostname())] = true
			d.mu.Unlock()
		}
		return u, err
	}
}

func (d *downloadDialer) trusted(host string) bool {
	host = strings.ToLower(host)
	if host == d.trustedHost {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.proxyHosts[host]
}

// DialContext implements http.Transport.DialContext.
func (d *downloadDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if d.trusted(host) {
		return d.dialer.DialContext(ctx, network, addr)
	}
	guarded := *d.dialer
	guarded.Control = func(_, address string, _ syscall.RawConn) error {
		ipStr, _, err := net.SplitHostPort(address)
		if err != nil {
			return err
		}
		ip := net.ParseIP(ipStr)
		if ip == nil || d.blocked(ip) {
			return fmt.Errorf("%w (%s resolved to %s)", errBlockedAddress, host, ipStr)
		}
		return nil
	}
	return guarded.DialContext(ctx, network, addr)
}

// newDownloadTransport clones base for attachment downloads, routing every
// connection through a downloadDialer that trusts only server's host.
func newDownloadTransport(base *http.Transport, server *url.URL) (*http.Transport, *downloadDialer) {
	d := newDownloadDialer(server.Hostname())
	t := base.Clone()
	t.Proxy = d.wrapProxy(base.Proxy)
	t.DialContext = d.DialContext
	return t, d
}
