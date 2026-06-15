package api

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func hostOf(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("parse %q: %v", raw, err)
	}
	return u.Host
}

func TestAuthTransport_AttachesKeyOnConfiguredHost(t *testing.T) {
	var gotKey string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Redmine-API-Key")
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()

	client := &http.Client{Transport: &authTransport{
		base: http.DefaultTransport, authMethod: "apikey", apiKey: "K", host: hostOf(t, ts.URL),
	}}
	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if gotKey != "K" {
		t.Errorf("X-Redmine-API-Key = %q, want K (configured host must receive auth)", gotKey)
	}
}

func TestAuthTransport_StripsKeyOnForeignHost(t *testing.T) {
	var gotKey string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Redmine-API-Key")
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()

	// Configured host is deliberately different from the server we call.
	client := &http.Client{Transport: &authTransport{
		base: http.DefaultTransport, authMethod: "apikey", apiKey: "K", host: "redmine.internal:9999",
	}}
	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if gotKey != "" {
		t.Errorf("X-Redmine-API-Key = %q, want empty (must not leak to a foreign host)", gotKey)
	}
}

func TestAuthTransport_StripsBasicAuthOnForeignHost(t *testing.T) {
	var gotUser string
	var hadAuth bool
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUser, _, hadAuth = r.BasicAuth()
		_, _ = w.Write([]byte("{}"))
	}))
	defer ts.Close()

	client := &http.Client{Transport: &authTransport{
		base: http.DefaultTransport, authMethod: "basic", username: "u", password: "p", host: "redmine.internal:9999",
	}}
	req, _ := http.NewRequest(http.MethodGet, ts.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	resp.Body.Close()
	if hadAuth {
		t.Errorf("basic auth leaked to foreign host (user=%q)", gotUser)
	}
}

// TestAttachmentDownload_CrossHostRedirectDoesNotLeakKey reproduces the
// realistic case where an attachment's content_url 302-redirects to external
// object storage. The API key must reach the configured server but never the
// off-host redirect target.
func TestAttachmentDownload_CrossHostRedirectDoesNotLeakKey(t *testing.T) {
	var evilGotKey string
	evil := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		evilGotKey = r.Header.Get("X-Redmine-API-Key")
		_, _ = w.Write([]byte("payload"))
	}))
	defer evil.Close()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Redmine-API-Key") != "SECRET" {
			t.Errorf("configured host missing API key: %q", r.Header.Get("X-Redmine-API-Key"))
		}
		http.Redirect(w, r, evil.URL+"/object-storage", http.StatusFound)
	}))
	defer srv.Close()

	c := &Client{
		httpClient: &http.Client{Transport: &authTransport{
			base: http.DefaultTransport, authMethod: "apikey", apiKey: "SECRET", host: hostOf(t, srv.URL),
		}},
		baseURL:  srv.URL,
		debugLog: debug.New(nil),
	}
	c.Attachments = &AttachmentService{client: c}

	att := &models.Attachment{ID: 1, Filename: "x.bin", ContentURL: srv.URL + "/attachments/download/1/x.bin"}
	var buf bytes.Buffer
	if _, err := c.Attachments.Download(context.Background(), att, &buf); err != nil {
		t.Fatalf("download failed: %v", err)
	}
	if evilGotKey != "" {
		t.Fatalf("API key leaked to off-host redirect target: %q", evilGotKey)
	}
	if buf.String() != "payload" {
		t.Errorf("body = %q, want payload (redirect must still be followed)", buf.String())
	}
}
