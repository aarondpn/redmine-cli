package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// tlsTestTransport returns a transport configured like NewClient's that
// trusts the shared httptest certificate for every host name (the
// certificate is issued for example.com, 127.0.0.1 and ::1).
func tlsTestTransport(t *testing.T, ts *httptest.Server) *http.Transport {
	t.Helper()
	base := newBaseTransport(defaultResponseHeaderTimeout)
	tlsCfg := ts.Client().Transport.(*http.Transport).TLSClientConfig.Clone()
	tlsCfg.ServerName = "example.com"
	base.TLSClientConfig = tlsCfg
	return base
}

func TestIsCleartextServer(t *testing.T) {
	tests := []struct {
		server string
		want   bool
	}{
		{"http://redmine.example.com", true},
		{"HTTP://redmine.example.com/", true},
		{"http://10.0.0.5:3000", true},
		{"http://user:pw@redmine.example.com", true},
		{"https://redmine.example.com", false},
		{"http://localhost:3000", false},
		{"http://LOCALHOST", false},
		{"http://127.0.0.1:3000", false},
		{"http://127.8.9.10", false},
		{"http://[::1]:3000", false},
		{"http://localhost.:3000", false},
		{"http://dev.localhost:3000", false},
		{"http://localhost.example.com", true},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsCleartextServer(tt.server); got != tt.want {
			t.Errorf("IsCleartextServer(%q) = %v, want %v", tt.server, got, tt.want)
		}
	}
}

func TestIsLoopbackHost(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"localhost", true},
		{"LOCALHOST", true},
		{"localhost.", true},
		{"dev.localhost", true},
		{"Dev.LocalHost.", true},
		{"127.0.0.1", true},
		{"127.8.9.10", true},
		{"::1", true},
		{"::ffff:127.0.0.1", true},
		{"localhost.example.com", false},
		{"mylocalhost", false},
		{"10.0.0.1", false},
		{"0.0.0.0", false},
		{"::", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsLoopbackHost(tt.host); got != tt.want {
			t.Errorf("IsLoopbackHost(%q) = %v, want %v", tt.host, got, tt.want)
		}
	}
}

func TestCleartextServerWarning_ScrubsUserinfo(t *testing.T) {
	msg := CleartextServerWarning("http://alice:hunter2@redmine.example.com")
	if strings.Contains(msg, "hunter2") {
		t.Errorf("warning %q leaks the URL password", msg)
	}
	if !strings.Contains(msg, "cleartext") {
		t.Errorf("warning %q should mention cleartext", msg)
	}
}

func TestResponseHeaderTimeoutEnv(t *testing.T) {
	tests := []struct {
		value   string
		want    time.Duration
		wantErr bool
	}{
		{"", 0, false},
		{"0", 0, false},
		{"45s", 45 * time.Second, false},
		{" 5m ", 5 * time.Minute, false},
		{"soon", 0, true},
		{"-1s", 0, true},
		{"30", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			t.Setenv(HTTPTimeoutEnv, tt.value)
			got, err := responseHeaderTimeout()
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("timeout = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewClient_InvalidHTTPTimeout(t *testing.T) {
	t.Setenv(HTTPTimeoutEnv, "soon")
	_, err := NewClient(&config.Config{Server: "https://redmine.example.com"}, debug.New(nil))
	if err == nil || !strings.Contains(err.Error(), HTTPTimeoutEnv) {
		t.Fatalf("err = %v, want an error naming %s", err, HTTPTimeoutEnv)
	}
}

func TestNewClient_TransportTimeouts(t *testing.T) {
	t.Setenv(HTTPTimeoutEnv, "")
	c, err := NewClient(&config.Config{Server: "https://redmine.example.com"}, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if c.httpClient.Timeout != 0 {
		t.Errorf("http.Client.Timeout must stay unset so large downloads can stream")
	}
	tr := c.httpClient.Transport.(*authTransport).base.(*http.Transport)
	// Slow queries must keep working: the wait for response headers is only
	// bounded when REDMINE_HTTP_TIMEOUT asks for it.
	if tr.ResponseHeaderTimeout != 0 {
		t.Errorf("ResponseHeaderTimeout = %v, want 0 (disabled by default)", tr.ResponseHeaderTimeout)
	}
	if tr.TLSHandshakeTimeout != 15*time.Second {
		t.Errorf("TLSHandshakeTimeout = %v, want 15s", tr.TLSHandshakeTimeout)
	}
	if tr.IdleConnTimeout != 90*time.Second {
		t.Errorf("IdleConnTimeout = %v, want 90s", tr.IdleConnTimeout)
	}
	if tr.Proxy == nil {
		t.Errorf("transport lost proxy-from-environment support")
	}
}

func TestNewClient_ResponseHeaderTimeout(t *testing.T) {
	release := make(chan struct{})
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()
	defer close(release)

	t.Setenv(HTTPTimeoutEnv, "50ms")
	c, err := NewClient(&config.Config{Server: ts.URL}, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	err = c.Get(context.Background(), "/issues.json", nil, &out)
	if err == nil || !strings.Contains(err.Error(), "timeout awaiting response headers") {
		t.Fatalf("err = %v, want a response header timeout", err)
	}
}

func TestAuthTransport_SchemeMismatchDropsCredentials(t *testing.T) {
	var got http.Header
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()

	// Same host:port as the server, but the configured origin is https.
	u, _ := url.Parse(ts.URL)
	client := &http.Client{Transport: &authTransport{
		base: http.DefaultTransport, authMethod: "apikey", apiKey: "K",
		origin:  &config.Origin{Scheme: "https", Host: u.Hostname(), Port: u.Port()},
		headers: map[string]string{"X-Proxy-Token": "secret"},
	}}
	resp, err := client.Get(ts.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if v := got.Get("X-Redmine-API-Key"); v != "" {
		t.Errorf("X-Redmine-API-Key = %q, must not be sent over http to an https origin", v)
	}
	if v := got.Get("X-Proxy-Token"); v != "" {
		t.Errorf("X-Proxy-Token = %q, must not be sent over http to an https origin", v)
	}
}

func TestAuthTransport_DefaultPortMatchesConfiguredOrigin(t *testing.T) {
	var gotKey string
	rt := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		gotKey = r.Header.Get("X-Redmine-API-Key")
		return &http.Response{StatusCode: http.StatusOK, Body: http.NoBody, Request: r}, nil
	})
	server, _ := url.Parse("https://Redmine.Example.com")
	client := &http.Client{Transport: &authTransport{base: rt, apiKey: "K", origin: originFor(t, server.String())}}
	resp, err := client.Get("https://redmine.example.com:443/issues.json")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if gotKey != "K" {
		t.Errorf("X-Redmine-API-Key = %q, want K for the same origin with an explicit default port", gotKey)
	}
}

func TestAuthTransport_AttachesTo(t *testing.T) {
	tests := []struct {
		server, target string
		want           bool
	}{
		{"https://redmine.example.com", "https://redmine.example.com/issues.json", true},
		{"https://redmine.example.com", "https://REDMINE.example.com:443/x", true},
		{"https://redmine.example.com.", "https://redmine.example.com/x", true},
		{"https://redmine.example.com", "https://redmine.example.com./x", true},
		{"https://redmine.example.com", "http://redmine.example.com/x", false},
		{"https://redmine.example.com", "https://redmine.example.com:8443/x", false},
		{"https://redmine.example.com", "https://storage.example.com/x", false},
		{"https://redmine.example.com:8443", "http://redmine.example.com:8443/x", false},
		// An http:// server that upgrades to https keeps working.
		{"http://redmine.example.com", "https://redmine.example.com/x", true},
		{"http://redmine.example.com:8080", "https://redmine.example.com:8080/x", true},
		{"http://redmine.example.com", "https://redmine.example.com:8443/x", false},
		{"http://redmine.example.com", "https://evil.example.com/x", false},
	}
	for _, tt := range tests {
		server, _ := url.Parse(tt.server)
		target, _ := url.Parse(tt.target)
		tr := &authTransport{origin: originFor(t, server.String())}
		if got := tr.attachesTo(target); got != tt.want {
			t.Errorf("server %s: attachesTo(%s) = %v, want %v", tt.server, tt.target, got, tt.want)
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// TestClient_RefusesHTTPSToHTTPRedirect reproduces the M-01 audit PoC: an
// https server redirecting to plain http must not cause the request (and the
// API key) to be resent in cleartext.
func TestClient_RefusesHTTPSToHTTPRedirect(t *testing.T) {
	var plainHits atomic.Int32
	var plainKey atomic.Value
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		plainHits.Add(1)
		plainKey.Store(r.Header.Get("X-Redmine-API-Key"))
		_, _ = w.Write([]byte("{}"))
	}))
	defer plain.Close()

	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL+r.URL.Path, http.StatusFound)
	}))
	defer secure.Close()

	cfg := &config.Config{Server: secure.URL, AuthMethod: "apikey", APIKey: "SUPER_SECRET_API_KEY"}
	c := newClient(cfg, debug.New(nil), tlsTestTransport(t, secure))

	var out map[string]any
	err := c.Get(context.Background(), "/issues.json", nil, &out)
	if err == nil || !strings.Contains(err.Error(), "refusing redirect") {
		t.Fatalf("err = %v, want refused https->http redirect", err)
	}
	if n := plainHits.Load(); n != 0 {
		t.Fatalf("plaintext server received %d request(s) (key %v); the redirect must not be followed", n, plainKey.Load())
	}
}

func TestClient_SameOriginHTTPSRedirectKeepsCredentials(t *testing.T) {
	var finalKey string
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old/issues.json" {
			http.Redirect(w, r, "/issues.json", http.StatusMovedPermanently)
			return
		}
		finalKey = r.Header.Get("X-Redmine-API-Key")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer secure.Close()

	cfg := &config.Config{Server: secure.URL, AuthMethod: "apikey", APIKey: "K"}
	c := newClient(cfg, debug.New(nil), tlsTestTransport(t, secure))

	var out map[string]any
	if err := c.Get(context.Background(), "/old/issues.json", nil, &out); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if finalKey != "K" {
		t.Errorf("X-Redmine-API-Key after same-origin redirect = %q, want K", finalKey)
	}
}

func TestClient_StopsAfterTenRedirects(t *testing.T) {
	var hits atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.Redirect(w, r, "/loop", http.StatusFound)
	}))
	defer ts.Close()

	c, err := NewClient(&config.Config{Server: ts.URL}, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	err = c.Get(context.Background(), "/loop", nil, &out)
	if err == nil || !strings.Contains(err.Error(), "stopped after 10 redirects") {
		t.Fatalf("err = %v, want redirect cap error", err)
	}
	if n := hits.Load(); n != 10 {
		t.Errorf("server hits = %d, want 10", n)
	}
}

func TestClient_RefusedRedirectErrorScrubsURL(t *testing.T) {
	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://storage.example.com/obj?X-Amz-Signature=TOPSECRET&key=APIKEY", http.StatusFound)
	}))
	defer secure.Close()

	c := newClient(&config.Config{Server: secure.URL}, debug.New(nil), tlsTestTransport(t, secure))
	var out map[string]any
	err := c.Get(context.Background(), "/issues.json", nil, &out)
	if err == nil {
		t.Fatal("expected the downgrade redirect to be refused")
	}
	if msg := err.Error(); strings.Contains(msg, "TOPSECRET") || strings.Contains(msg, "APIKEY") {
		t.Errorf("error leaks secrets from the redirect URL: %q", msg)
	}
}
