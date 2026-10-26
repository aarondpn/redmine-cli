package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// stubLookup replaces lookupIPAddr for the duration of the test.
func stubLookup(t *testing.T, addrs map[string]string) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	orig := lookupIPAddr
	lookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		calls.Add(1)
		ip, ok := addrs[host]
		if !ok {
			return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
		}
		return []net.IPAddr{{IP: net.ParseIP(ip)}}, nil
	}
	t.Cleanup(func() { lookupIPAddr = orig })
	return &calls
}

// TestClient_ProxiedRedirectToInternalHostRefused reproduces a redirect that
// travels through a forward proxy: the guarded dialer only sees the proxy, so
// the target host name must be resolved and checked by the redirect policy.
func TestClient_ProxiedRedirectToInternalHostRefused(t *testing.T) {
	stubLookup(t, map[string]string{
		"internal.test": "10.0.0.5",
		"public.test":   "93.184.216.34",
	})

	var proxied atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxied.Add(1)
		_, _ = w.Write([]byte(`{"via":"` + r.URL.Host + `"}`))
	}))
	defer proxy.Close()
	proxyURL, _ := url.Parse(proxy.URL)

	redmine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://"+strings.TrimPrefix(r.URL.Path, "/to/")+"/meta-data", http.StatusFound)
	}))
	defer redmine.Close()

	base := newBaseTransport(0)
	base.Proxy = func(r *http.Request) (*url.URL, error) {
		if strings.HasSuffix(r.URL.Hostname(), ".test") || r.URL.Hostname() == "no-dns.example" {
			return proxyURL, nil
		}
		return nil, nil
	}
	c := newClient(&config.Config{Server: redmine.URL, APIKey: "K"}, debug.New(nil), base)

	var out map[string]any
	err := c.Get(context.Background(), "/to/internal.test", nil, &out)
	if err == nil || !strings.Contains(err.Error(), "non-public address") {
		t.Fatalf("err = %v, want refusal of a proxied redirect to an internal host", err)
	}
	if n := proxied.Load(); n != 0 {
		t.Fatalf("proxy received %d request(s); the redirect must not be followed", n)
	}

	// Public targets, and names only the proxy can resolve, still work.
	for _, target := range []string{"public.test", "no-dns.example"} {
		out = nil
		if err := c.Get(context.Background(), "/to/"+target, nil, &out); err != nil {
			t.Fatalf("redirect to %s: %v", target, err)
		}
		if out["via"] != target {
			t.Fatalf("redirect to %s answered %v, want it served through the proxy", target, out)
		}
	}
}

func TestRedirectPolicy_ResolvesOnlyProxiedHostNames(t *testing.T) {
	calls := stubLookup(t, map[string]string{"internal.test": "10.0.0.5"})
	server, _ := url.Parse("https://redmine.example.com")
	prev := httptest.NewRequest(http.MethodGet, "https://redmine.example.com/x", nil)
	req := httptest.NewRequest(http.MethodGet, "https://internal.test/x", nil)

	// Without a proxy the guarded dialer checks the resolved address itself.
	if err := redirectPolicy(server, nil)(req, []*http.Request{prev}); err != nil {
		t.Fatalf("unproxied redirect refused by the policy: %v", err)
	}
	noProxy := func(*http.Request) (*url.URL, error) { return nil, nil }
	if err := redirectPolicy(server, noProxy)(req, []*http.Request{prev}); err != nil {
		t.Fatalf("unproxied redirect refused by the policy: %v", err)
	}
	if n := calls.Load(); n != 0 {
		t.Fatalf("lookup called %d time(s) for unproxied redirects", n)
	}

	proxyErr := func(*http.Request) (*url.URL, error) { return nil, errors.New("bad proxy config") }
	if err := redirectPolicy(server, proxyErr)(req, []*http.Request{prev}); err != nil {
		t.Fatalf("redirect refused although no proxy was chosen: %v", err)
	}
}
