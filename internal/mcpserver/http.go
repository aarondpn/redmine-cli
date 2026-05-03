package mcpserver

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
)

// HTTPOptions controls how BuildHTTPServer assembles the streamable HTTP
// transport. Zero values yield safe defaults suitable for a localhost bind.
type HTTPOptions struct {
	// Addr is the listen address passed to http.Server. Required.
	Addr string

	// AuthToken, when non-empty, requires every request to carry an
	// `Authorization: Bearer <token>` header that matches it. Compared in
	// constant time.
	AuthToken string

	// ReadHeaderTimeout, ReadTimeout, WriteTimeout, IdleTimeout override the
	// http.Server timeouts when non-zero. Defaults are applied when left as
	// the zero value.
	ReadHeaderTimeout time.Duration
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// Default timeouts applied when HTTPOptions leaves them at zero. WriteTimeout
// stays at zero because tool calls can take a while (slow Redmine instances,
// large attachments) and the streamable handler does not need a write deadline
// to be safe.
const (
	defaultReadHeaderTimeout = 10 * time.Second
	defaultReadTimeout       = 30 * time.Second
	defaultIdleTimeout       = 120 * time.Second
)

// BuildHTTPHandler constructs a streamable HTTP handler backed by the same MCP
// server definition used for stdio. It does not apply any auth or timeouts on
// its own — wrap it with WrapAuthToken and host it on a configured
// http.Server, or use BuildHTTPServer to get both in one call.
func BuildHTTPHandler(client *api.Client, opts Options) http.Handler {
	srv := BuildServer(client, opts)
	return mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return srv
	}, &mcp.StreamableHTTPOptions{
		JSONResponse: true,
	})
}

// BuildHTTPServer returns a fully configured *http.Server that exposes the
// MCP transport with hardened timeouts and an optional bearer-token gate.
// Caller is responsible for calling ListenAndServe / Shutdown.
func BuildHTTPServer(client *api.Client, opts Options, httpOpts HTTPOptions) *http.Server {
	handler := BuildHTTPHandler(client, opts)
	if httpOpts.AuthToken != "" {
		handler = WrapAuthToken(handler, httpOpts.AuthToken)
	}

	readHeader := httpOpts.ReadHeaderTimeout
	if readHeader == 0 {
		readHeader = defaultReadHeaderTimeout
	}
	read := httpOpts.ReadTimeout
	if read == 0 {
		read = defaultReadTimeout
	}
	idle := httpOpts.IdleTimeout
	if idle == 0 {
		idle = defaultIdleTimeout
	}

	return &http.Server{
		Addr:              httpOpts.Addr,
		Handler:           handler,
		ReadHeaderTimeout: readHeader,
		ReadTimeout:       read,
		WriteTimeout:      httpOpts.WriteTimeout,
		IdleTimeout:       idle,
	}
}

// WrapAuthToken gates h behind a constant-time bearer-token check. An empty
// token disables the gate (the handler is returned untouched). Failed checks
// respond 401 with a WWW-Authenticate header so MCP clients can retry with
// credentials.
func WrapAuthToken(h http.Handler, token string) http.Handler {
	if token == "" {
		return h
	}
	expected := []byte(token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fields := strings.Fields(r.Header.Get("Authorization"))
		if len(fields) != 2 || !strings.EqualFold(fields[0], "Bearer") {
			unauthorized(w, "missing bearer token")
			return
		}
		got := []byte(fields[1])
		if subtle.ConstantTimeCompare(got, expected) != 1 {
			unauthorized(w, "invalid bearer token")
			return
		}
		h.ServeHTTP(w, r)
	})
}

func unauthorized(w http.ResponseWriter, msg string) {
	w.Header().Set("WWW-Authenticate", `Bearer realm="redmine-cli"`)
	http.Error(w, msg, http.StatusUnauthorized)
}
