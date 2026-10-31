// Package oauthtest provides a fake Redmine OAuth provider that behaves like
// Doorkeeper: confidential clients, PKCE S256, loopback redirects that ignore
// the port, and rotating refresh tokens.
package oauthtest

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// grant is one issued token pair.
type grant struct {
	access, refresh string
	scopes          []string
	revoked         bool   // the refresh token (and pair) is revoked
	expired         bool   // the access token is past its lifetime
	previous        *grant // revoked once this access token is first used
}

type authCode struct {
	scopes      []string
	redirectURI string
	challenge   string
}

// Server is a fake Redmine with Doorkeeper's /oauth endpoints.
type Server struct {
	*httptest.Server
	// Mux serves everything; tests add Redmine endpoints to it.
	Mux          *http.ServeMux
	ClientID     string
	ClientSecret string
	// ExpiresIn is the expires_in of issued tokens, in seconds.
	ExpiresIn int
	// AllowedScopes limits the scopes the application is registered with;
	// nil allows any.
	AllowedScopes []string
	// Refreshes and Revokes count successful refresh_token grants and
	// revocations.
	Refreshes, Revokes atomic.Int32

	mu      sync.Mutex
	codes   map[string]authCode
	byRefr  map[string]*grant
	byAcces map[string]*grant
}

// NewServer starts a fake provider that is closed when the test ends.
func NewServer(t *testing.T) *Server {
	t.Helper()
	s := &Server{
		Mux:          http.NewServeMux(),
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		ExpiresIn:    7200,
		codes:        map[string]authCode{},
		byRefr:       map[string]*grant{},
		byAcces:      map[string]*grant{},
	}
	s.Mux.HandleFunc("POST /oauth/token", s.handleToken)
	s.Mux.HandleFunc("POST /oauth/revoke", s.handleRevoke)
	s.Mux.Handle("GET /users/current.json", s.RequireBearer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"user": map[string]any{"id": 1, "login": "tester"}})
	})))
	s.Server = httptest.NewServer(s.Mux)
	t.Cleanup(s.Close)
	return s
}

// Approve plays the browser: it validates the authorize request, mints a code
// (or an invalid_scope error) and GETs the redirect URI.
func (s *Server) Approve(t *testing.T, authURL string) {
	t.Helper()
	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if q.Get("response_type") != "code" || q.Get("client_id") != s.ClientID {
		t.Fatalf("bad authorize request: %s", u.RawQuery)
	}
	if q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		t.Fatalf("authorize request without S256 challenge: %s", u.RawQuery)
	}
	redirect := q.Get("redirect_uri")
	if !sameRedirect(redirect, "http://127.0.0.1/callback") {
		t.Fatalf("redirect_uri %q is not registered", redirect)
	}
	scopes := strings.Fields(q.Get("scope"))
	back, err := url.Parse(redirect)
	if err != nil {
		t.Fatal(err)
	}
	bq := back.Query()
	bq.Set("state", q.Get("state"))
	if s.AllowedScopes != nil && !subset(scopes, s.AllowedScopes) {
		bq.Set("error", "invalid_scope")
	} else {
		code := randomToken()
		s.mu.Lock()
		s.codes[code] = authCode{scopes: scopes, redirectURI: redirect, challenge: q.Get("code_challenge")}
		s.mu.Unlock()
		bq.Set("code", code)
	}
	back.RawQuery = bq.Encode()
	resp, err := http.Get(back.String())
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
}

// Issue creates a token pair directly, as if a login had happened.
func (s *Server) Issue(scopes []string) (access, refresh string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.newGrant(scopes, nil)
	return g.access, g.refresh
}

// ExpireAccessTokens makes every issued access token expired: Redmine answers
// 401 until the client refreshes.
func (s *Server) ExpireAccessTokens() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, g := range s.byAcces {
		g.expired = true
	}
}

// RequireBearer rejects requests without a live access token the way Redmine
// does: 401 with a Bearer invalid_token challenge.
func (s *Server) RequireBearer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		s.mu.Lock()
		g := s.byAcces[tok]
		live := ok && g != nil && !g.revoked && !g.expired
		if live && g.previous != nil {
			// First use of the new access token revokes the old pair.
			g.previous.revoked = true
			g.previous = nil
		}
		s.mu.Unlock()
		if !live {
			w.Header().Set("WWW-Authenticate", `Bearer realm="Doorkeeper", error="invalid_token", error_description="The access token is invalid"`)
			http.Error(w, "", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// newGrant must be called with s.mu held.
func (s *Server) newGrant(scopes []string, previous *grant) *grant {
	g := &grant{access: randomToken(), refresh: randomToken(), scopes: scopes, previous: previous}
	s.byAcces[g.access] = g
	s.byRefr[g.refresh] = g
	return g
}

func (s *Server) clientAuth(w http.ResponseWriter, r *http.Request) bool {
	id, secret, ok := r.BasicAuth()
	if ok {
		id, _ = url.QueryUnescape(id)
		secret, _ = url.QueryUnescape(secret)
	}
	if !ok || id != s.ClientID || secret != s.ClientSecret {
		w.Header().Set("WWW-Authenticate", `Basic realm="Doorkeeper"`)
		oauthError(w, http.StatusUnauthorized, "invalid_client")
		return false
	}
	return true
}

func (s *Server) handleToken(w http.ResponseWriter, r *http.Request) {
	if !s.clientAuth(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var g *grant
	switch r.PostForm.Get("grant_type") {
	case "authorization_code":
		c, ok := s.codes[r.PostForm.Get("code")]
		delete(s.codes, r.PostForm.Get("code"))
		sum := sha256.Sum256([]byte(r.PostForm.Get("code_verifier")))
		verified := base64.RawURLEncoding.EncodeToString(sum[:]) == c.challenge
		if !ok || !verified || !sameRedirect(r.PostForm.Get("redirect_uri"), c.redirectURI) {
			oauthError(w, http.StatusBadRequest, "invalid_grant")
			return
		}
		g = s.newGrant(c.scopes, nil)
	case "refresh_token":
		old := s.byRefr[r.PostForm.Get("refresh_token")]
		if old == nil || old.revoked {
			oauthError(w, http.StatusBadRequest, "invalid_grant")
			return
		}
		g = s.newGrant(old.scopes, old)
		s.Refreshes.Add(1)
	default:
		oauthError(w, http.StatusBadRequest, "unsupported_grant_type")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"access_token":  g.access,
		"token_type":    "Bearer",
		"expires_in":    s.ExpiresIn,
		"refresh_token": g.refresh,
		"scope":         strings.Join(g.scopes, " "),
	})
}

func (s *Server) handleRevoke(w http.ResponseWriter, r *http.Request) {
	if !s.clientAuth(w, r) {
		return
	}
	if err := r.ParseForm(); err != nil {
		oauthError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	tok := r.PostForm.Get("token")
	s.mu.Lock()
	defer s.mu.Unlock()
	if g := s.byRefr[tok]; g != nil {
		g.revoked = true
	} else if g := s.byAcces[tok]; g != nil {
		g.revoked = true
	}
	s.Revokes.Add(1)
	writeJSON(w, http.StatusOK, map[string]any{})
}

// sameRedirect compares redirect URIs the way Doorkeeper does for loopback IP
// literals: the port is ignored.
func sameRedirect(got, want string) bool {
	g, err1 := url.Parse(got)
	w, err2 := url.Parse(want)
	if err1 != nil || err2 != nil {
		return false
	}
	gh, wh := g.Hostname(), w.Hostname()
	if ip := net.ParseIP(gh); ip != nil && ip.IsLoopback() && gh == wh {
		return g.Scheme == w.Scheme && g.Path == w.Path
	}
	return got == want
}

func subset(scopes, allowed []string) bool {
	for _, sc := range scopes {
		if !slices.Contains(allowed, sc) {
			return false
		}
	}
	return true
}

func randomToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func oauthError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, map[string]any{"error": code, "error_description": code})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
