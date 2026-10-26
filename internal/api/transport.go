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
	"unicode/utf8"

	"golang.org/x/net/idna"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// HTTPTimeoutEnv names the environment variable that sets how long the
// client waits for a server's response headers. The value is a Go duration
// ("90s", "5m"); "0" or unset means no limit.
const HTTPTimeoutEnv = "REDMINE_HTTP_TIMEOUT"

// Transport timeouts. There is deliberately no overall http.Client.Timeout:
// it would also cap the time spent streaming a response body, which breaks
// large attachment downloads on slow links. Connection setup is bounded,
// since a working server never needs that long. The wait for response
// headers is not bounded by default: Redmine can legitimately take minutes
// to answer an expensive query, so it is opt-in via HTTPTimeoutEnv.
const (
	dialTimeout                  = 30 * time.Second
	tlsHandshakeTimeout          = 15 * time.Second
	defaultResponseHeaderTimeout = time.Duration(0)
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
// support from the environment) with bounded dial, TLS handshake and idle
// connection timeouts, and the given response header timeout (0 disables it).
func newBaseTransport(headerTimeout time.Duration) *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DialContext = (&net.Dialer{Timeout: dialTimeout, KeepAlive: 30 * time.Second}).DialContext
	t.TLSHandshakeTimeout = tlsHandshakeTimeout
	t.ResponseHeaderTimeout = headerTimeout
	t.IdleConnTimeout = idleConnTimeout
	return t
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
	return !IsLoopbackHost(u.Hostname())
}

// CleartextServerWarning returns the security warning shown for a plain
// http:// server (see IsCleartextServer).
func CleartextServerWarning(server string) string {
	return fmt.Sprintf("server %s uses plain http://, so the API key or password is sent in cleartext; use an https:// URL", debug.ScrubURL(server))
}

// IsLoopbackHost reports whether host, a URL host name or IP literal without
// port or brackets, names this machine: "localhost" or a name under it
// ("dev.localhost", reserved for loopback by RFC 6761), or a loopback IP
// (127.0.0.0/8, ::1). Case and a trailing dot are ignored.
func IsLoopbackHost(host string) bool {
	host = config.NormalizeHost(host)
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// checkRedirect keeps net/http's limit of 10 redirects and refuses any
// redirect that downgrades https to http, so the request (and anything the
// server put in the URL) is never resent in cleartext. Credentials are
// handled separately by authTransport, which only attaches them to the
// configured origin.
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

// redirectPolicy returns the redirect policy of every request, API calls and
// attachment downloads alike. On top of checkRedirect (redirect cap, no
// https->http downgrade) it only follows redirects that
//   - stay on the configured server: its origin, or the https upgrade of an
//     http:// server (see isServerOrigin). Another port on the same host
//     may be a different, internal service, so it counts as elsewhere; or
//   - go to another origin that is not a non-public address (see
//     isBlockedIP). This keeps installations that offload attachments to
//     object storage (e.g. S3 presigned URLs) or move to another public host
//     working without turning the CLI into a proxy for internal services.
//     Plain http:// is allowed there as long as the redirect does not
//     downgrade from https; no credentials are sent off the configured
//     origin either way.
//
// Literal IPs are checked here; host names are re-checked against the IP
// actually dialed by guardedDialer, which defeats DNS rebinding. A redirect
// that proxy sends through a proxy never reaches guardedDialer with its own
// address, so its host name is resolved here instead (see
// checkProxiedRedirect). (net/http reports the refused target URL alongside
// these errors.)
func redirectPolicy(server *url.URL, proxy func(*http.Request) (*url.URL, error)) func(*http.Request, []*http.Request) error {
	serverOrigin := config.OriginOf(server)
	return func(req *http.Request, via []*http.Request) error {
		if err := checkRedirect(req, via); err != nil {
			return err
		}
		if isServerOrigin(serverOrigin, config.OriginOf(req.URL)) {
			return nil
		}
		host := req.URL.Hostname()
		if ip := net.ParseIP(host); ip != nil {
			if isBlockedIP(ip) {
				return fmt.Errorf("refusing redirect: %s is a local, private or otherwise non-public address", ip)
			}
			return nil
		}
		if looksNumericIPv4(host) {
			// Shorthand IPv4 forms such as 127.1 or 2130706433 are not
			// canonical IPs to net.ParseIP, yet many resolvers and proxies
			// map them to addresses; no real host name ends in a number.
			return fmt.Errorf("refusing redirect: %s is a non-canonical IP address", host)
		}
		if proxy != nil {
			if p, err := proxy(req); err == nil && p != nil {
				return checkProxiedRedirect(req.Context(), host)
			}
		}
		return nil
	}
}

// looksNumericIPv4 reports whether host's last dot-separated label is a
// decimal or 0x-hex number, which WHATWG URL parsing treats as an IPv4
// address (127.1, 2130706433, 0x7f.1, 10.1).
func looksNumericIPv4(host string) bool {
	label := strings.TrimSuffix(host, ".")
	if i := strings.LastIndexByte(label, '.'); i >= 0 {
		label = label[i+1:]
	}
	if label == "" {
		return false
	}
	digits := label
	if len(label) > 2 && (label[:2] == "0x" || label[:2] == "0X") {
		digits = label[2:]
		for _, c := range digits {
			if !strings.ContainsRune("0123456789abcdefABCDEF", c) {
				return false
			}
		}
		return true
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// lookupIPAddr resolves host names for checkProxiedRedirect; tests replace it.
var lookupIPAddr = net.DefaultResolver.LookupIPAddr

// checkProxiedRedirect refuses a redirect to host when host resolves to a
// non-public address. The proxy connects to the target itself, so the
// connect-time check in guardedDialer only ever sees the proxy. Resolving
// locally is best effort: the proxy may resolve the name differently, and a
// name this machine cannot resolve (proxy-only DNS, common in corporate
// networks) is left to the proxy, since refusing it would break ordinary
// object-storage redirects there.
func checkProxiedRedirect(ctx context.Context, host string) error {
	addrs, err := lookupIPAddr(ctx, host)
	if err != nil {
		return nil
	}
	for _, a := range addrs {
		if isBlockedIP(a.IP) {
			return fmt.Errorf("refusing redirect: %s resolves to %s, a local, private or otherwise non-public address", host, a.IP)
		}
	}
	return nil
}

// blockedPrefixes are non-public IPv4 ranges that the net/netip predicates in
// isBlockedAddr do not cover.
var blockedPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),     // "this network" (RFC 1122)
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT shared space (RFC 6598)
	netip.MustParsePrefix("198.18.0.0/15"), // benchmarking (RFC 2544)
}

// IPv6 prefixes that embed an IPv4 address a gateway may route to.
var (
	nat64Prefix  = netip.MustParsePrefix("64:ff9b::/96") // NAT64 (RFC 6052), IPv4 in the last 4 bytes
	sixToFourNet = netip.MustParsePrefix("2002::/16")    // 6to4 (RFC 3056), IPv4 in bytes 2-5
)

// isBlockedIP reports whether ip is an address a redirect away from the
// configured server must not reach: loopback, private (RFC 1918, RFC 4193),
// link-local (169.254.0.0/16 including cloud metadata endpoints, fe80::/10),
// unspecified, multicast, 0.0.0.0/8, carrier-grade NAT (100.64.0.0/10) or
// benchmarking (198.18.0.0/15) space. IPv6 addresses that embed an IPv4
// address (IPv4-mapped ::ffff:0:0/96, NAT64 64:ff9b::/96, 6to4 2002::/16) are
// judged by the embedded address. An unparseable ip is blocked.
func isBlockedIP(ip net.IP) bool {
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return true
	}
	return isBlockedAddr(addr)
}

func isBlockedAddr(addr netip.Addr) bool {
	addr = addr.Unmap()
	if addr.Is6() {
		b := addr.As16()
		switch {
		case nat64Prefix.Contains(addr):
			return isBlockedAddr(netip.AddrFrom4([4]byte(b[12:16])))
		case sixToFourNet.Contains(addr):
			return isBlockedAddr(netip.AddrFrom4([4]byte(b[2:6])))
		}
	}
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() ||
		addr.IsLinkLocalMulticast() || addr.IsInterfaceLocalMulticast() ||
		addr.IsMulticast() || addr.IsUnspecified() {
		return true
	}
	for _, p := range blockedPrefixes {
		if p.Contains(addr) {
			return true
		}
	}
	return false
}

// errBlockedAddress is returned when a connection would reach an address
// rejected by isBlockedIP.
var errBlockedAddress = errors.New("refusing to connect: redirect target resolves to a local, private or otherwise non-public address")

// guardedDialer dials every connection of the client. Connections to the
// configured server (the host:port endpoints redirectPolicy treats as the
// server, see isServerOrigin), and to any proxy the transport chose, are
// dialed normally (a local or intranet Redmine is expected to live on a
// private address). Every other connection can only come from a redirect and
// is checked at connect time: the resolved IP must pass blocked == false, so
// a host name that re-resolves to an internal address after the redirect
// check is still refused.
type guardedDialer struct {
	dialer  *net.Dialer
	trusted map[string]bool // normalized host:port endpoints
	blocked func(net.IP) bool

	mu      sync.Mutex
	proxies map[string]bool
}

func newGuardedDialer(server config.Origin) *guardedDialer {
	d := &guardedDialer{
		dialer:  &net.Dialer{Timeout: dialTimeout, KeepAlive: 30 * time.Second},
		trusted: map[string]bool{},
		blocked: isBlockedIP,
		proxies: map[string]bool{},
	}
	ports := []string{server.Port}
	if server.Scheme == "http" && server.Port == "80" {
		ports = append(ports, "443") // the https upgrade
	}
	for _, h := range dialHosts(server.Host) {
		for _, port := range ports {
			d.trusted[endpoint(h, port)] = true
		}
	}
	return d
}

// endpoint returns the normalized host:port key for a dialed address.
func endpoint(host, port string) string {
	return net.JoinHostPort(config.NormalizeHost(host), port)
}

// dialHosts returns the normalized forms in which net/http may dial host: as
// written, and, for an internationalized name, the ASCII (punycode) form
// net/http converts it to.
func dialHosts(host string) []string {
	hosts := []string{config.NormalizeHost(host)}
	if isASCII(host) {
		return hosts
	}
	if ascii, err := idna.Lookup.ToASCII(host); err == nil {
		if a := config.NormalizeHost(ascii); a != hosts[0] {
			hosts = append(hosts, a)
		}
	}
	return hosts
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// proxyPorts are the default ports of the proxy schemes net/http supports.
var proxyPorts = map[string]string{"http": "80", "https": "443", "socks5": "1080", "socks5h": "1080"}

// wrapProxy records the endpoints of proxies returned by proxy so
// DialContext lets the transport reach them.
func (d *guardedDialer) wrapProxy(proxy func(*http.Request) (*url.URL, error)) func(*http.Request) (*url.URL, error) {
	if proxy == nil {
		return nil
	}
	return func(req *http.Request) (*url.URL, error) {
		u, err := proxy(req)
		if u != nil {
			port := u.Port()
			if port == "" {
				port = proxyPorts[strings.ToLower(u.Scheme)]
			}
			d.mu.Lock()
			for _, h := range dialHosts(u.Hostname()) {
				d.proxies[endpoint(h, port)] = true
			}
			d.mu.Unlock()
		}
		return u, err
	}
}

func (d *guardedDialer) isTrusted(host, port string) bool {
	ep := endpoint(host, port)
	if d.trusted[ep] {
		return true
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.proxies[ep]
}

// DialContext implements http.Transport.DialContext.
func (d *guardedDialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if d.isTrusted(host, port) {
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

// newGuardedTransport clones base, routing every connection through a
// guardedDialer that trusts only the configured server's endpoints.
func newGuardedTransport(base *http.Transport, server *url.URL) (*http.Transport, *guardedDialer) {
	d := newGuardedDialer(config.OriginOf(server))
	t := base.Clone()
	t.Proxy = d.wrapProxy(base.Proxy)
	t.DialContext = d.DialContext
	return t, d
}
