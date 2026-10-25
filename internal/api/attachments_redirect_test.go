package api

import (
	"bytes"
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
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// redirectingServer returns a server that redirects every request to target.
func redirectingServer(target string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target, http.StatusFound)
	}))
}

func downloadAttachment(t *testing.T, c *Client) (string, error) {
	t.Helper()
	att := &models.Attachment{ID: 1, Filename: "x.bin"}
	var buf bytes.Buffer
	_, err := c.Attachments.Download(context.Background(), att, &buf)
	return buf.String(), err
}

// TestAttachmentDownload_RefusesRedirectToInternalAddress reproduces the M-10
// audit PoC: a Redmine server redirecting an attachment download to cloud
// metadata, loopback or private addresses must not be followed.
func TestAttachmentDownload_RefusesRedirectToInternalAddress(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		wantErr string
	}{
		{"cloud metadata over http", "http://169.254.169.254/latest/meta-data/", "only https"},
		{"cloud metadata over https", "https://169.254.169.254/latest/meta-data/", "non-public address"},
		{"rfc1918 10/8", "https://10.0.0.1/internal", "non-public address"},
		{"rfc1918 172.16/12", "https://172.16.0.1/internal", "non-public address"},
		{"rfc1918 192.168/16", "https://192.168.1.10/internal", "non-public address"},
		{"other loopback", "https://127.0.0.2/admin", "non-public address"},
		{"ipv6 loopback", "https://[::1]:8443/admin", "non-public address"},
		{"ipv6 unique local", "https://[fd00::1]/internal", "non-public address"},
		{"ipv6 link-local", "https://[fe80::1]/internal", "non-public address"},
		{"ipv4-mapped private", "https://[::ffff:10.0.0.1]/internal", "non-public address"},
		{"unspecified", "https://0.0.0.0/", "non-public address"},
		{"multicast", "https://224.0.0.1/", "non-public address"},
		{"carrier-grade nat", "https://100.64.0.1/", "non-public address"},
		{"this network", "https://0.1.2.3/", "non-public address"},
		{"benchmarking", "https://198.18.0.1/", "non-public address"},
		{"nat64 metadata", "https://[64:ff9b::a9fe:a9fe]/latest/meta-data/", "non-public address"},
		{"6to4 private", "https://[2002:a00:1::1]/internal", "non-public address"},
		{"public host over http", "http://storage.example.com/obj", "only https"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := redirectingServer(tt.target)
			defer srv.Close()

			c, err := NewClient(&config.Config{Server: srv.URL, AuthMethod: "apikey", APIKey: "K"}, debug.New(nil))
			if err != nil {
				t.Fatal(err)
			}
			_, err = downloadAttachment(t, c)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("err = %v, want refusal containing %q", err, tt.wantErr)
			}
		})
	}
}

// TestAttachmentDownload_RedirectRecheckedAtDial covers DNS rebinding: a
// redirect to a host name passes the URL check, but the address it resolves
// to is re-checked when connecting.
func TestAttachmentDownload_RedirectRecheckedAtDial(t *testing.T) {
	var targetHits atomic.Int32
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		targetHits.Add(1)
		_, _ = w.Write([]byte("internal"))
	}))
	defer target.Close()

	// "localhost" is not the configured host (127.0.0.1) and resolves to a
	// loopback address.
	srv := redirectingServer(strings.Replace(target.URL, "127.0.0.1", "localhost", 1) + "/obj")
	defer srv.Close()

	c := newClient(&config.Config{Server: srv.URL, APIKey: "K"}, debug.New(nil), tlsTestTransport(t, target))
	_, err := downloadAttachment(t, c)
	if !errors.Is(err, errBlockedAddress) {
		t.Fatalf("err = %v, want errBlockedAddress", err)
	}
	if n := targetHits.Load(); n != 0 {
		t.Errorf("internal target received %d request(s)", n)
	}
}

// TestAttachmentDownload_FollowsPublicHTTPSRedirectWithoutCredentials keeps
// external object storage (e.g. S3 presigned URLs) working.
func TestAttachmentDownload_FollowsPublicHTTPSRedirectWithoutCredentials(t *testing.T) {
	var storageKey atomic.Value
	storage := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		storageKey.Store(r.Header.Get("X-Redmine-API-Key"))
		_, _ = w.Write([]byte("payload"))
	}))
	defer storage.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Redmine-API-Key") != "K" {
			t.Errorf("configured server missing API key")
		}
		http.Redirect(w, r, strings.Replace(storage.URL, "127.0.0.1", "localhost", 1)+"/bucket/x.bin?X-Amz-Signature=abc", http.StatusFound)
	}))
	defer srv.Close()

	c := newClient(&config.Config{Server: srv.URL, APIKey: "K"}, debug.New(nil), tlsTestTransport(t, storage))
	// Treat loopback as public so the test server stands in for S3.
	c.downloadDialer.blocked = func(net.IP) bool { return false }

	body, err := downloadAttachment(t, c)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if body != "payload" {
		t.Errorf("body = %q, want payload", body)
	}
	if k, _ := storageKey.Load().(string); k != "" {
		t.Errorf("API key leaked to object storage: %q", k)
	}
}

func TestAttachmentDownload_SameOriginRedirectKeepsCredentials(t *testing.T) {
	var finalKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/attachments/download/") {
			http.Redirect(w, r, "/files/x.bin", http.StatusFound)
			return
		}
		finalKey = r.Header.Get("X-Redmine-API-Key")
		_, _ = w.Write([]byte("payload"))
	}))
	defer srv.Close()

	c, err := NewClient(&config.Config{Server: srv.URL, AuthMethod: "apikey", APIKey: "K"}, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	body, err := downloadAttachment(t, c)
	if err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if body != "payload" || finalKey != "K" {
		t.Errorf("body = %q, key = %q; want payload with the API key on the same origin", body, finalKey)
	}
}

// TestAttachmentDownload_PrivateServerSameHostRedirect: a Redmine on a
// private or loopback address may redirect within its own host.
func TestAttachmentDownload_PrivateServerSameHostRedirect(t *testing.T) {
	sibling := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("payload"))
	}))
	defer sibling.Close()

	srv := redirectingServer(sibling.URL + "/x.bin")
	defer srv.Close()

	c, err := NewClient(&config.Config{Server: srv.URL, APIKey: "K"}, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	if body, err := downloadAttachment(t, c); err != nil || body != "payload" {
		t.Fatalf("body = %q, err = %v; want payload", body, err)
	}
}

func TestAttachmentDownload_RefusesHTTPSDowngrade(t *testing.T) {
	var plainHits atomic.Int32
	plain := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		plainHits.Add(1)
		_, _ = w.Write([]byte("payload"))
	}))
	defer plain.Close()

	secure := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, plain.URL+"/x.bin", http.StatusFound)
	}))
	defer secure.Close()

	c := newClient(&config.Config{Server: secure.URL, APIKey: "K"}, debug.New(nil), tlsTestTransport(t, secure))
	_, err := downloadAttachment(t, c)
	if err == nil || !strings.Contains(err.Error(), "insecure") {
		t.Fatalf("err = %v, want refused https->http redirect", err)
	}
	if n := plainHits.Load(); n != 0 {
		t.Errorf("plaintext server received %d request(s)", n)
	}
}

// The configured server may live in any of the blocked ranges (an intranet
// Redmine on CGNAT or a NAT64 address); redirects within its host are still
// followed, while the same ranges stay off-limits for every other host.
func TestDownloadRedirectPolicy_SameHostInBlockedRange(t *testing.T) {
	tests := []struct {
		server, target string
		allowed        bool
	}{
		{"https://100.64.1.1", "https://100.64.1.1:8443/files/x.bin", true},
		{"https://[64:ff9b::a00:1]", "https://[64:ff9b::a00:1]/files/x.bin", true},
		{"https://198.18.0.10", "https://198.18.0.10/files/x.bin", true},
		{"https://100.64.1.1", "https://100.64.1.2/files/x.bin", false},
		{"https://redmine.example.com", "https://[2002:a00:1::1]/x.bin", false},
		{"https://redmine.example.com", "https://[::ffff:100.64.0.1]/x.bin", false},
	}
	for _, tt := range tests {
		t.Run(tt.server+" -> "+tt.target, func(t *testing.T) {
			server, err := url.Parse(tt.server)
			if err != nil {
				t.Fatal(err)
			}
			prev := httptest.NewRequest(http.MethodGet, tt.server+"/attachments/download/1", nil)
			req := httptest.NewRequest(http.MethodGet, tt.target, nil)
			err = downloadRedirectPolicy(server)(req, []*http.Request{prev})
			if tt.allowed && err != nil {
				t.Fatalf("redirect refused: %v", err)
			}
			if !tt.allowed && (err == nil || !strings.Contains(err.Error(), "non-public address")) {
				t.Fatalf("err = %v, want refusal of a non-public address", err)
			}
		})
	}
}

func TestIsBlockedDownloadIP(t *testing.T) {
	tests := []struct {
		ip      string
		blocked bool
	}{
		{"127.0.0.1", true},
		{"10.1.2.3", true},
		{"172.31.255.255", true},
		{"192.168.0.1", true},
		{"169.254.169.254", true},
		{"0.0.0.0", true},
		{"224.0.0.251", true},
		{"::1", true},
		{"::", true},
		{"fc00::1", true},
		{"fe80::1", true},
		{"ff02::1", true},
		{"0.1.2.3", true},
		{"100.64.0.1", true},
		{"100.127.255.255", true},
		{"198.18.0.1", true},
		{"198.19.255.255", true},
		{"::ffff:10.0.0.1", true},
		{"::ffff:100.64.0.1", true},
		{"::ffff:8.8.8.8", false},
		{"64:ff9b::7f00:1", true},    // NAT64 of 127.0.0.1
		{"64:ff9b::a9fe:a9fe", true}, // NAT64 of 169.254.169.254
		{"64:ff9b::808:808", false},  // NAT64 of 8.8.8.8
		{"2002:c0a8:101::1", true},   // 6to4 of 192.168.1.1
		{"2002:6440:1::1", true},     // 6to4 of 100.64.0.1
		{"2002:808:808::1", false},   // 6to4 of 8.8.8.8
		{"8.8.8.8", false},
		{"52.216.0.1", false},
		{"172.32.0.1", false},
		{"100.63.255.255", false},
		{"100.128.0.0", false},
		{"198.17.255.255", false},
		{"198.20.0.0", false},
		{"1.0.0.1", false},
		{"2600:1f18::1", false},
	}
	for _, tt := range tests {
		if got := isBlockedDownloadIP(net.ParseIP(tt.ip)); got != tt.blocked {
			t.Errorf("isBlockedDownloadIP(%s) = %v, want %v", tt.ip, got, tt.blocked)
		}
	}
}
