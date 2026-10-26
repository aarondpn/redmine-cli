package mcpserver

import (
	"net"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
)

// NormalizeBindAddr applies the convenience defaults used by `redmine mcp
// serve --http <addr>`:
//
//   - An empty string returns "" with isLoopback=false (caller skips HTTP).
//   - An address starting with ":" (no host) is rewritten to bind on
//     127.0.0.1 only. Listening on all interfaces is opt-in, never the
//     default.
//   - Explicit hosts are returned unchanged. The boolean reports whether the
//     resolved host is a loopback address; callers use it to decide whether
//     to warn about exposing the server without an auth token.
//
// Unparsable addresses are returned as-is with isLoopback=false; the bind
// will fail downstream with a clearer error.
func NormalizeBindAddr(addr string) (normalized string, isLoopback bool) {
	if addr == "" {
		return "", false
	}
	if strings.HasPrefix(addr, ":") {
		return "127.0.0.1" + addr, true
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr, false
	}
	if host == "" {
		return net.JoinHostPort("127.0.0.1", port), true
	}
	return addr, api.IsLoopbackHost(host)
}
