package config

import (
	"net"
	"net/url"
	"strings"
)

// ServerOrigin returns the normalized origin (scheme://host[:port]) of a server
// URL: scheme and host are lowercased and default ports are dropped. A URL
// without a scheme is treated as https, mirroring SameServerHost. It returns ""
// when raw has no host.
func ServerOrigin(raw string) string {
	raw = strings.TrimSpace(raw)
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		u, err = url.Parse("https://" + raw)
		if err != nil || u.Host == "" {
			return ""
		}
	}
	scheme := strings.ToLower(u.Scheme)
	host := strings.ToLower(u.Hostname())
	port := u.Port()
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		port = ""
	}
	if port != "" {
		host = net.JoinHostPort(host, port)
	} else if strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	return scheme + "://" + host
}

// SameOrigin reports whether two server URLs share scheme, host and port.
func SameOrigin(a, b string) bool {
	oa := ServerOrigin(a)
	return oa != "" && oa == ServerOrigin(b)
}
