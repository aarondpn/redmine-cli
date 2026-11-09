package ghrelease

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// clearTokens unsets both token variables, so a token in the developer's or
// CI's environment cannot leak into a test.
func clearTokens(t *testing.T) {
	t.Helper()
	t.Setenv("GITHUB_TOKEN", "")
	t.Setenv("GH_TOKEN", "")
}

func TestToken(t *testing.T) {
	tests := []struct {
		name        string
		githubToken string
		ghToken     string
		want        string
	}{
		{name: "none", want: ""},
		{name: "GITHUB_TOKEN", githubToken: "a", want: "a"},
		{name: "GH_TOKEN", ghToken: "b", want: "b"},
		{name: "GITHUB_TOKEN wins", githubToken: "a", ghToken: "b", want: "a"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GITHUB_TOKEN", tt.githubToken)
			t.Setenv("GH_TOKEN", tt.ghToken)
			if got := Token(); got != tt.want {
				t.Errorf("Token() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNewHTTPClientHasTimeouts(t *testing.T) {
	c := NewHTTPClient()
	if c.Timeout != RequestTimeout {
		t.Errorf("Timeout = %v, want %v", c.Timeout, RequestTimeout)
	}
	transport, ok := c.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("Transport = %T, want *http.Transport", c.Transport)
	}
	if transport.ResponseHeaderTimeout != ResponseHeaderTimeout || transport.TLSHandshakeTimeout <= 0 {
		t.Errorf("transport timeouts: response header %v, TLS handshake %v", transport.ResponseHeaderTimeout, transport.TLSHandshakeTimeout)
	}
	if transport == http.DefaultTransport {
		t.Error("NewHTTPClient must not modify the shared default transport")
	}
}

func TestReleaseURL(t *testing.T) {
	tests := []struct {
		name       string
		base       string
		owner      string
		repo       string
		tag        string
		want       string
		wantErrSub string
	}{
		{name: "latest", owner: "octo", repo: "redmine-cli-pr", want: "https://api.github.com/repos/octo/redmine-cli-pr/releases/latest"},
		{name: "tag", owner: "octo", repo: "redmine-cli-pr", tag: "v1.2.3", want: "https://api.github.com/repos/octo/redmine-cli-pr/releases/tags/v1.2.3"},
		{name: "tag is path escaped", owner: "octo", repo: "r", tag: "release/1 2?x", want: "https://api.github.com/repos/octo/r/releases/tags/release%2F1%202%3Fx"},
		{name: "custom base with trailing slash", base: "http://127.0.0.1:1234/api/", owner: "o", repo: "r", want: "http://127.0.0.1:1234/api/repos/o/r/releases/latest"},
		{name: "dotted repo", owner: "o", repo: ".github", want: "https://api.github.com/repos/o/.github/releases/latest"},
		{name: "empty owner", owner: "", repo: "r", wantErrSub: "invalid GitHub owner"},
		{name: "owner with slash", owner: "o/x", repo: "r", wantErrSub: "invalid GitHub owner"},
		{name: "repo dot dot", owner: "o", repo: "..", wantErrSub: "invalid GitHub repository"},
		{name: "repo with query", owner: "o", repo: "r?x=1", wantErrSub: "invalid GitHub repository"},
		{name: "repo too long", owner: "o", repo: strings.Repeat("r", 101), wantErrSub: "invalid GitHub repository"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{APIBaseURL: tt.base}
			got, err := c.ReleaseURL(tt.owner, tt.repo, tt.tag)
			if tt.wantErrSub != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErrSub) {
					t.Fatalf("ReleaseURL() error = %v, want %q", err, tt.wantErrSub)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("ReleaseURL() = %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestNilClientUsesDefaults(t *testing.T) {
	var c *Client
	if got := c.APIURL("/x"); got != DefaultAPIBaseURL+"/x" {
		t.Errorf("APIURL() = %q", got)
	}
	if c.httpClient() != defaultHTTPClient {
		t.Error("nil client must use the shared default HTTP client")
	}
}

func TestFetchRelease(t *testing.T) {
	clearTokens(t)
	var gotPath, gotAccept, gotVersion string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		gotAccept = r.Header.Get("Accept")
		gotVersion = r.Header.Get("X-GitHub-Api-Version")
		switch r.URL.Path {
		case "/repos/octo/redmine-cli-pr/releases/latest":
			fmt.Fprint(w, `{"tag_name":"v2.0.0","name":"Two","html_url":"https://github.com/octo/redmine-cli-pr/releases/tag/v2.0.0",
				"published_at":"2026-01-02T03:04:05Z","prerelease":false,
				"assets":[{"name":"redmine-cli-pr-linux-amd64","size":42,"browser_download_url":"https://example.com/a"},
				          {"name":"checksums.txt","browser_download_url":"https://example.com/c"}]}`)
		case "/repos/octo/redmine-cli-pr/releases/tags/v1.0.0":
			fmt.Fprint(w, `{"tag_name":"v1.0.0","assets":[]}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	c := &Client{HTTPClient: ts.Client(), APIBaseURL: ts.URL}

	rel, err := c.FetchRelease(context.Background(), "octo", "redmine-cli-pr", "")
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/repos/octo/redmine-cli-pr/releases/latest" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAccept != "application/vnd.github+json" || gotVersion != "2022-11-28" {
		t.Errorf("Accept = %q, X-GitHub-Api-Version = %q", gotAccept, gotVersion)
	}
	if rel.TagName != "v2.0.0" || rel.Name != "Two" || len(rel.Assets) != 2 {
		t.Fatalf("release = %+v", rel)
	}
	if want := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC); !rel.PublishedAt.Equal(want) {
		t.Errorf("PublishedAt = %v, want %v", rel.PublishedAt, want)
	}
	asset, ok := rel.FindAsset("redmine-cli-pr-linux-amd64")
	if !ok || asset.Size != 42 || asset.BrowserDownloadURL != "https://example.com/a" {
		t.Errorf("FindAsset() = %+v, %v", asset, ok)
	}
	if _, ok := rel.FindAsset("redmine-cli-pr-linux"); ok {
		t.Error("FindAsset() must match the exact name only")
	}

	rel, err = c.FetchRelease(context.Background(), "octo", "redmine-cli-pr", "v1.0.0")
	if err != nil || rel.TagName != "v1.0.0" {
		t.Fatalf("tagged release = %+v, %v", rel, err)
	}

	_, err = c.FetchRelease(context.Background(), "octo", "no-releases", "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing release error = %v, want ErrNotFound", err)
	}
	var se *StatusError
	if !errors.As(err, &se) || se.StatusCode != http.StatusNotFound {
		t.Fatalf("missing release error = %#v, want a 404 StatusError", err)
	}
	if err.Error() != "GitHub API returned status 404" {
		t.Errorf("404 message = %q", err.Error())
	}

	if _, err := c.FetchRelease(context.Background(), "bad/owner", "r", ""); err == nil {
		t.Fatal("expected an error for an invalid owner")
	}
}

func TestFindAssetNilRelease(t *testing.T) {
	var r *Release
	if _, ok := r.FindAsset("x"); ok {
		t.Fatal("nil release must have no assets")
	}
}

func TestGetJSONStatusErrors(t *testing.T) {
	tests := []struct {
		status   int
		wantMsg  string
		notFound bool
	}{
		{status: http.StatusForbidden, wantMsg: "GitHub API returned status 403 (rate limited? set GITHUB_TOKEN or GH_TOKEN)"},
		{status: http.StatusTooManyRequests, wantMsg: "GitHub API returned status 429 (rate limited? set GITHUB_TOKEN or GH_TOKEN)"},
		{status: http.StatusUnauthorized, wantMsg: "GitHub API returned status 401 (GITHUB_TOKEN or GH_TOKEN rejected; unset it or use a valid token)"},
		{status: http.StatusNotFound, wantMsg: "GitHub API returned status 404", notFound: true},
		{status: http.StatusInternalServerError, wantMsg: "GitHub API returned status 500"},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			clearTokens(t)
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
			}))
			defer ts.Close()

			var v map[string]any
			err := (&Client{HTTPClient: ts.Client()}).GetJSON(context.Background(), ts.URL, 1024, &v)
			if err == nil || err.Error() != tt.wantMsg {
				t.Fatalf("GetJSON() error = %v, want %q", err, tt.wantMsg)
			}
			if errors.Is(err, ErrNotFound) != tt.notFound {
				t.Errorf("errors.Is(err, ErrNotFound) = %v, want %v", !tt.notFound, tt.notFound)
			}
		})
	}
}

func TestGetJSONTokenHandling(t *testing.T) {
	tests := []struct {
		name       string
		token      string
		rejectAuth bool
		wantAuths  []string
		wantErr    bool
	}{
		{name: "anonymous", wantAuths: []string{""}},
		{name: "token sent", token: "good", wantAuths: []string{"Bearer good"}},
		{name: "rejected token retried anonymously", token: "stale", rejectAuth: true, wantAuths: []string{"Bearer stale", ""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearTokens(t)
			t.Setenv("GH_TOKEN", tt.token)
			var mu sync.Mutex
			var auths []string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				auths = append(auths, r.Header.Get("Authorization"))
				mu.Unlock()
				if tt.rejectAuth && r.Header.Get("Authorization") != "" {
					w.WriteHeader(http.StatusUnauthorized)
					return
				}
				fmt.Fprint(w, `{"tag_name":"v1"}`)
			}))
			defer ts.Close()

			rel, err := (&Client{HTTPClient: ts.Client()}).FetchReleaseURL(context.Background(), ts.URL)
			if err != nil || rel.TagName != "v1" {
				t.Fatalf("FetchReleaseURL() = %+v, %v", rel, err)
			}
			if fmt.Sprint(auths) != fmt.Sprint(tt.wantAuths) {
				t.Errorf("Authorization headers = %q, want %q", auths, tt.wantAuths)
			}
		})
	}
}

func TestGetJSONRejectsOversizedAndInvalidBodies(t *testing.T) {
	clearTokens(t)
	body := `{"tag_name":"` + strings.Repeat("v", 100) + `"}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/chunked" {
			w.(http.Flusher).Flush()
		}
		if r.URL.Path == "/invalid" {
			fmt.Fprint(w, "not json")
			return
		}
		fmt.Fprint(w, body)
	}))
	defer ts.Close()
	c := &Client{HTTPClient: ts.Client()}

	for _, path := range []string{"/declared", "/chunked"} {
		var v Release
		err := c.GetJSON(context.Background(), ts.URL+path, 50, &v)
		var tooLarge *ResponseTooLargeError
		if !errors.As(err, &tooLarge) || tooLarge.Limit != 50 {
			t.Errorf("%s: GetJSON() error = %v, want ResponseTooLargeError{50}", path, err)
		}
	}
	var v Release
	if err := c.GetJSON(context.Background(), ts.URL+"/fits", int64(len(body)), &v); err != nil {
		t.Errorf("body at the limit: %v", err)
	}
	if err := c.GetJSON(context.Background(), ts.URL+"/invalid", 1024, &v); err == nil {
		t.Error("expected a decode error for a non-JSON body")
	}
}

func TestDownloadLimit(t *testing.T) {
	const limit = 1024
	tests := []struct {
		name      string
		size      int
		chunked   bool
		status    int
		wantError string
		tooLarge  bool
	}{
		{name: "at the limit", size: limit},
		{name: "empty", size: 0},
		{name: "declared length over the limit", size: limit + 1, tooLarge: true},
		{name: "chunked body over the limit", size: limit + 1, chunked: true, tooLarge: true},
		{name: "not found", status: http.StatusNotFound, wantError: "download returned status 404"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clearTokens(t)
			t.Setenv("GITHUB_TOKEN", "secret")
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Header.Get("Authorization") != "" {
					t.Error("asset downloads must not carry the GitHub token")
				}
				if tt.status != 0 {
					w.WriteHeader(tt.status)
					return
				}
				if tt.chunked {
					// Flushing before the body forces chunked encoding,
					// so no Content-Length reveals the size up front.
					w.(http.Flusher).Flush()
				}
				_, _ = w.Write(bytes.Repeat([]byte("a"), tt.size))
			}))
			defer ts.Close()

			data, err := (&Client{HTTPClient: ts.Client()}).Download(context.Background(), ts.URL, limit)
			switch {
			case tt.tooLarge:
				var tooLarge *ResponseTooLargeError
				if !errors.As(err, &tooLarge) {
					t.Fatalf("Download() error = %v, want ResponseTooLargeError", err)
				}
			case tt.wantError != "":
				if err == nil || err.Error() != tt.wantError {
					t.Fatalf("Download() error = %v, want %q", err, tt.wantError)
				}
			default:
				if err != nil || len(data) != tt.size {
					t.Fatalf("Download() = %d bytes, %v; want %d bytes", len(data), err, tt.size)
				}
			}
		})
	}
}

func TestDownloadHonorsContext(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("asset"))
	}))
	defer ts.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := (&Client{HTTPClient: ts.Client()}).Download(ctx, ts.URL, 1024); !errors.Is(err, context.Canceled) {
		t.Fatalf("Download() error = %v, want context.Canceled", err)
	}
}

func TestLookupChecksum(t *testing.T) {
	file := []byte("aaaa  other.tar.gz\n" +
		"bbbb  redmine-cli-pr-linux-amd64\n" +
		"cccc *redmine-cli-pr-windows-amd64.exe\n" +
		"malformed line with extra fields\n" +
		"dddd  redmine-cli-pr-linux-amd64\n")
	tests := []struct {
		name   string
		asset  string
		want   string
		wantOK bool
	}{
		{name: "first match wins", asset: "redmine-cli-pr-linux-amd64", want: "bbbb", wantOK: true},
		{name: "binary mode marker", asset: "redmine-cli-pr-windows-amd64.exe", want: "cccc", wantOK: true},
		{name: "missing", asset: "redmine-cli-pr-darwin-arm64", wantOK: false},
		{name: "no prefix match", asset: "other", wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := LookupChecksum(file, tt.asset)
			if got != tt.want || ok != tt.wantOK {
				t.Errorf("LookupChecksum(%q) = %q, %v; want %q, %v", tt.asset, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestVerifySHA256(t *testing.T) {
	data := []byte("hello world")
	sum := SHA256Hex(data)
	if sum != "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9" {
		t.Fatalf("SHA256Hex() = %q", sum)
	}
	tests := []struct {
		name      string
		checksums string
		wantErr   string
	}{
		{name: "match", checksums: sum + "  asset\n"},
		{name: "uppercase hash", checksums: strings.ToUpper(sum) + "  asset\n"},
		{name: "mismatch", checksums: strings.Repeat("0", 64) + "  asset\n", wantErr: "expected " + strings.Repeat("0", 64) + ", got " + sum},
		{name: "missing", checksums: sum + "  other\n", wantErr: "no checksum found for asset"},
		{name: "empty file", checksums: "", wantErr: "no checksum found for asset"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := VerifySHA256(data, []byte(tt.checksums), "asset")
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("VerifySHA256() = %v", err)
				}
				return
			}
			if err == nil || err.Error() != tt.wantErr {
				t.Fatalf("VerifySHA256() = %v, want %q", err, tt.wantErr)
			}
		})
	}
}

func TestVerifyChecksum(t *testing.T) {
	data := []byte("hello world")
	line := SHA256Hex(data) + "  asset\n"
	tests := []struct {
		name     string
		handler  http.HandlerFunc
		wantErr  string
		tooLarge bool
	}{
		{name: "valid", handler: func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, line) }},
		{name: "server error", handler: func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) }, wantErr: "checksums download returned status 500"},
		{name: "mismatch", handler: func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, strings.Repeat("0", 64)+"  asset\n") }, wantErr: "expected "},
		{
			name: "oversized file with early match",
			handler: func(w http.ResponseWriter, r *http.Request) {
				// The cap must apply to the whole response, not only to
				// the part scanned before a match.
				_, _ = io.WriteString(w, line)
				_, _ = w.Write(bytes.Repeat([]byte("x"), MaxChecksumsBytes))
			},
			tooLarge: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ts := httptest.NewServer(tt.handler)
			defer ts.Close()
			err := (&Client{HTTPClient: ts.Client()}).VerifyChecksum(context.Background(), data, "asset", ts.URL)
			switch {
			case tt.tooLarge:
				var tooLarge *ResponseTooLargeError
				if !errors.As(err, &tooLarge) || !strings.HasPrefix(err.Error(), "downloading checksums: ") {
					t.Fatalf("VerifyChecksum() = %v, want a wrapped ResponseTooLargeError", err)
				}
			case tt.wantErr != "":
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("VerifyChecksum() = %v, want %q", err, tt.wantErr)
				}
			default:
				if err != nil {
					t.Fatalf("VerifyChecksum() = %v", err)
				}
			}
		})
	}
}

func TestResponseTooLargeErrorMessage(t *testing.T) {
	err := &ResponseTooLargeError{Limit: 10}
	if got := err.Error(); got != "response is larger than 10 bytes; refusing to read it" {
		t.Errorf("Error() = %q", got)
	}
}
