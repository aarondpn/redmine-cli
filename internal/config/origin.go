package config

import (
	"net"
	"net/url"
	"strings"
)

// Origin is a URL's normalized scheme, host and port: the one definition of
// "the same server" used for credentials, redirects, links and profile
// changes. Compare Origins with ==.
type Origin struct {
	// Scheme is lowercased.
	Scheme string
	// Host is the host name (see NormalizeHost) or IP literal, without
	// brackets.
	Host string
	// Port is the explicit port, or the scheme's default ("443" for https,
	// "80" for http) when the URL has none; "" for other schemes without a
	// port.
	Port string
}

// OriginOf returns u's Origin, so "HTTPS://Redmine.Example.com." and
// "https://redmine.example.com:443" have the same one.
func OriginOf(u *url.URL) Origin {
	scheme := strings.ToLower(u.Scheme)
	port := u.Port()
	if port == "" {
		port = defaultPort(scheme)
	}
	return Origin{Scheme: scheme, Host: NormalizeHost(u.Hostname()), Port: port}
}

// NormalizeHost lowercases a host name and drops the trailing dot of a fully
// qualified name, so "Redmine.Example.com." and "redmine.example.com" compare
// equal.
func NormalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

func defaultPort(scheme string) string {
	switch scheme {
	case "https":
		return "443"
	case "http":
		return "80"
	}
	return ""
}

// HasDefaultPort reports whether o's port is its scheme's default.
func (o Origin) HasDefaultPort() bool {
	return o.Port != "" && o.Port == defaultPort(o.Scheme)
}

// String renders o as scheme://host[:port], leaving out a default port.
func (o Origin) String() string {
	host := o.Host
	if o.Port != "" && !o.HasDefaultPort() {
		host = net.JoinHostPort(host, o.Port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return o.Scheme + "://" + host
}

// parseServerOrigin returns the Origin of a configured server URL. A URL
// without a scheme is treated as https. ok is false when raw has no host.
func parseServerOrigin(raw string) (Origin, bool) {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		u, err = url.Parse("https://" + raw)
		if err != nil || u.Host == "" {
			return Origin{}, false
		}
	}
	return OriginOf(u), true
}

// ServerOrigin returns the normalized origin (scheme://host[:port]) of a server
// URL (see OriginOf): scheme and host are lowercased, a trailing dot is
// dropped and default ports are left out. A URL without a scheme is treated
// as https. It returns "" when raw has no host.
func ServerOrigin(raw string) string {
	o, ok := parseServerOrigin(raw)
	if !ok {
		return ""
	}
	return o.String()
}

// SameOrigin reports whether two server URLs share scheme, host and port.
func SameOrigin(a, b string) bool {
	oa, ok := parseServerOrigin(a)
	if !ok {
		return false
	}
	ob, ok := parseServerOrigin(b)
	return ok && oa == ob
}
