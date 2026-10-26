package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// API requests follow the same destination policy as attachment downloads.

// TestClient_RefusesRedirectToInternalHost reproduces the review PoC: the
// server answers `redmine api` with a 302 to a service on localhost, which
// must not be fetched and returned.
func TestClient_RefusesRedirectToInternalHost(t *testing.T) {
	var internalHits atomic.Int32
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		internalHits.Add(1)
		_, _ = w.Write([]byte(`{"secret":"internal"}`))
	}))
	defer internal.Close()

	srv := redirectingServer(strings.Replace(internal.URL, "127.0.0.1", "localhost", 1) + "/admin.json")
	defer srv.Close()

	c, err := NewClient(&config.Config{Server: srv.URL, APIKey: "K"}, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.DoRaw(context.Background(), http.MethodGet, "/issues.json", nil, nil)
	if !errors.Is(err, errBlockedAddress) {
		t.Fatalf("DoRaw = %+v, %v; want errBlockedAddress", resp, err)
	}
	var out map[string]any
	if err := c.Get(context.Background(), "/issues.json", nil, &out); !errors.Is(err, errBlockedAddress) {
		t.Fatalf("Get err = %v, want errBlockedAddress", err)
	}
	if n := internalHits.Load(); n != 0 {
		t.Errorf("internal service received %d request(s)", n)
	}
}

func TestClient_RefusesRedirectToNonPublicIP(t *testing.T) {
	for _, target := range []string{
		"http://169.254.169.254/latest/meta-data/",
		"http://127.0.0.2/admin",
		"https://10.0.0.1/internal",
		"http://[::1]:1/admin",
	} {
		t.Run(target, func(t *testing.T) {
			srv := redirectingServer(target)
			defer srv.Close()

			c, err := NewClient(&config.Config{Server: srv.URL, APIKey: "K"}, debug.New(nil))
			if err != nil {
				t.Fatal(err)
			}
			_, err = c.DoRaw(context.Background(), http.MethodGet, "/issues.json", nil, nil)
			if err == nil || !strings.Contains(err.Error(), "non-public address") {
				t.Fatalf("err = %v, want refusal of a non-public address", err)
			}
		})
	}
}

// A server that moved to another public host keeps working; the new host
// gets no credentials.
func TestClient_FollowsPublicCrossOriginRedirectWithoutCredentials(t *testing.T) {
	var newKey atomic.Value
	moved := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		newKey.Store(r.Header.Get("X-Redmine-API-Key"))
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer moved.Close()

	srv := redirectingServer(strings.Replace(moved.URL, "127.0.0.1", "localhost", 1) + "/issues.json")
	defer srv.Close()

	c, err := NewClient(&config.Config{Server: srv.URL, APIKey: "K"}, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	// Treat loopback as public so the test server stands in for a public host.
	c.dialer.blocked = func(net.IP) bool { return false }

	resp, err := c.DoRaw(context.Background(), http.MethodGet, "/issues.json", nil, nil)
	if err != nil {
		t.Fatalf("DoRaw failed: %v", err)
	}
	if string(resp.Body) != `{"ok":true}` {
		t.Errorf("body = %q", resp.Body)
	}
	if k, _ := newKey.Load().(string); k != "" {
		t.Errorf("API key leaked to the redirect target: %q", k)
	}
}

// The configured server is dialed without address checks, whether it is
// given as a loopback IP or a host name that resolves to one.
func TestClient_ConfiguredServerOnLoopbackHostName(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/old.json" {
			http.Redirect(w, r, "/issues.json", http.StatusFound)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer ts.Close()

	c, err := NewClient(&config.Config{Server: strings.Replace(ts.URL, "127.0.0.1", "LocalHost", 1)}, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := c.Get(context.Background(), "/old.json", nil, &out); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if out["ok"] != true {
		t.Errorf("out = %v", out)
	}
}

func TestDialHosts(t *testing.T) {
	tests := []struct {
		host string
		want []string
	}{
		{"redmine.example.com", []string{"redmine.example.com"}},
		{"Redmine.Example.com.", []string{"redmine.example.com"}},
		{"10.0.0.5", []string{"10.0.0.5"}},
		{"::1", []string{"::1"}},
		// net/http dials internationalized names in their punycode form.
		{"bücher.example", []string{"bücher.example", "xn--bcher-kva.example"}},
	}
	for _, tt := range tests {
		if got := dialHosts(tt.host); !slices.Equal(got, tt.want) {
			t.Errorf("dialHosts(%q) = %q, want %q", tt.host, got, tt.want)
		}
	}
}
