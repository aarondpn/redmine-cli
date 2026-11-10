// Package ghrelease fetches GitHub release metadata and release assets for
// self-update and extension installs. Every request is bounded: the HTTP
// client has timeouts, and every body is read with a size cap, since a
// release server (or anything impersonating one) must not be able to hang
// the CLI or exhaust its memory.
package ghrelease

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

// DefaultAPIBaseURL is the root of the public GitHub REST API.
const DefaultAPIBaseURL = "https://api.github.com"

const (
	// ResponseHeaderTimeout bounds the wait for a server to start answering.
	ResponseHeaderTimeout = 30 * time.Second
	// RequestTimeout bounds a whole request, including reading the body.
	RequestTimeout = 5 * time.Minute
	// MaxChecksumsBytes bounds a checksums file, which has one short line
	// per release asset.
	MaxChecksumsBytes = 1 << 20
	// MaxMetadataBytes bounds a JSON API response such as release metadata.
	// A release with hundreds of assets is a few hundred KiB.
	MaxMetadataBytes = 16 << 20
)

// NewHTTPClient returns a client for GitHub API and release download
// requests. It never waits forever: the server must start answering within
// ResponseHeaderTimeout, and a whole request (body included) may take at
// most RequestTimeout. The transport is a clone of the default one, so
// proxy settings and dial/TLS handshake timeouts are kept.
func NewHTTPClient() *http.Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = ResponseHeaderTimeout
	return &http.Client{Transport: transport, Timeout: RequestTimeout}
}

// defaultHTTPClient serves Clients without their own HTTPClient.
var defaultHTTPClient = NewHTTPClient()

// Token returns the GitHub token from GITHUB_TOKEN, or else GH_TOKEN, or "".
// A token only raises the anonymous API rate limit, which shared IPs (CI
// runners, company networks) easily exhaust; public releases are readable
// without one.
func Token() string {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t
	}
	return os.Getenv("GH_TOKEN")
}

// Release is the subset of a GitHub release the CLI uses.
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name,omitempty"`
	HTMLURL     string    `json:"html_url,omitempty"`
	Draft       bool      `json:"draft,omitempty"`
	Prerelease  bool      `json:"prerelease,omitempty"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

// Asset is a file attached to a release.
type Asset struct {
	Name               string `json:"name"`
	Size               int64  `json:"size,omitempty"`
	BrowserDownloadURL string `json:"browser_download_url"`
}

// FindAsset returns the asset with exactly the given name.
func (r *Release) FindAsset(name string) (Asset, bool) {
	if r == nil {
		return Asset{}, false
	}
	for _, a := range r.Assets {
		if a.Name == name {
			return a, true
		}
	}
	return Asset{}, false
}

// ErrNotFound matches (via errors.Is) a GitHub API 404: an unknown
// repository or tag, or a repository without any published release.
var ErrNotFound = errors.New("not found")

// StatusError is a GitHub API response other than 200 OK.
type StatusError struct {
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("GitHub API returned status %d%s", e.StatusCode, StatusHint(e.StatusCode))
}

// Is makes errors.Is(err, ErrNotFound) true for a 404.
func (e *StatusError) Is(target error) bool {
	return target == ErrNotFound && e.StatusCode == http.StatusNotFound
}

// StatusHint returns a parenthesized hint, with a leading space, for GitHub
// API statuses that a token setting explains, or "".
func StatusHint(status int) string {
	switch status {
	case http.StatusForbidden, http.StatusTooManyRequests:
		return " (rate limited? set GITHUB_TOKEN or GH_TOKEN)"
	case http.StatusUnauthorized:
		return " (GITHUB_TOKEN or GH_TOKEN rejected; unset it or use a valid token)"
	}
	return ""
}

// ResponseTooLargeError reports a response that exceeded its size cap.
type ResponseTooLargeError struct {
	Limit int64
}

func (e *ResponseTooLargeError) Error() string {
	return fmt.Sprintf("response is larger than %d bytes; refusing to read it", e.Limit)
}

// Client talks to the GitHub REST API and downloads release assets. The zero
// value is ready to use against api.github.com.
type Client struct {
	// HTTPClient sends every request; nil uses a shared NewHTTPClient.
	HTTPClient *http.Client
	// APIBaseURL is the REST API root, e.g. an httptest server in tests;
	// "" means DefaultAPIBaseURL.
	APIBaseURL string
}

func (c *Client) httpClient() *http.Client {
	if c != nil && c.HTTPClient != nil {
		return c.HTTPClient
	}
	return defaultHTTPClient
}

// APIURL joins path (e.g. "/repos/o/r/releases/latest") onto the API base.
func (c *Client) APIURL(path string) string {
	base := DefaultAPIBaseURL
	if c != nil && c.APIBaseURL != "" {
		base = c.APIBaseURL
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

// repoPartPattern matches a GitHub owner or repository name. It keeps the
// parts from adding path segments or a query to the request URL.
var repoPartPattern = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,100}$`)

// ValidateRepo reports whether owner and repo are plausible GitHub names.
func ValidateRepo(owner, repo string) error {
	for _, part := range []struct{ kind, value string }{{"owner", owner}, {"repository", repo}} {
		if !repoPartPattern.MatchString(part.value) || part.value == "." || part.value == ".." {
			return fmt.Errorf("invalid GitHub %s name %q", part.kind, part.value)
		}
	}
	return nil
}

// ReleaseURL returns the API URL of the release with the given tag of
// owner/repo, or of its latest release when tag is "". GitHub's "latest" is
// the most recent non-draft, non-prerelease release.
func (c *Client) ReleaseURL(owner, repo, tag string) (string, error) {
	if err := ValidateRepo(owner, repo); err != nil {
		return "", err
	}
	path := "/repos/" + owner + "/" + repo + "/releases/"
	if tag == "" {
		path += "latest"
	} else {
		path += "tags/" + url.PathEscape(tag)
	}
	return c.APIURL(path), nil
}

// FetchRelease fetches the release with the given tag of owner/repo, or its
// latest release when tag is "". A missing repository, tag or release fails
// with an error matching ErrNotFound.
func (c *Client) FetchRelease(ctx context.Context, owner, repo, tag string) (*Release, error) {
	u, err := c.ReleaseURL(owner, repo, tag)
	if err != nil {
		return nil, err
	}
	return c.FetchReleaseURL(ctx, u)
}

// FetchReleaseURL fetches release metadata from a full API URL.
func (c *Client) FetchReleaseURL(ctx context.Context, rawURL string) (*Release, error) {
	var release Release
	if err := c.GetJSON(ctx, rawURL, MaxMetadataBytes, &release); err != nil {
		return nil, err
	}
	return &release, nil
}

// GetJSON fetches a GitHub API URL and decodes its JSON body into v, reading
// at most limit bytes. The request carries Token() when one is set; a token
// GitHub rejects (401) is dropped and the request retried once anonymously,
// so a stale token cannot break lookups of public data. A status other than
// 200 OK is returned as a *StatusError.
func (c *Client) GetJSON(ctx context.Context, rawURL string, limit int64, v any) error {
	token := Token()
	resp, err := c.getAPI(ctx, rawURL, token)
	if err == nil && resp.StatusCode == http.StatusUnauthorized && token != "" {
		resp.Body.Close()
		resp, err = c.getAPI(ctx, rawURL, "")
	}
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &StatusError{StatusCode: resp.StatusCode}
	}
	data, err := readLimited(resp, limit)
	if err != nil {
		return err
	}
	return json.NewDecoder(bytes.NewReader(data)).Decode(v)
}

func (c *Client) getAPI(ctx context.Context, rawURL, token string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return c.httpClient().Do(req)
}

// GetLimited fetches rawURL and returns the status code and, for 200 OK, the
// body. It fails with a *ResponseTooLargeError instead of reading more than
// limit bytes. No token is sent: release assets are served from (and
// redirect to) hosts other than the API.
func (c *Client) GetLimited(ctx context.Context, rawURL string, limit int64) (int, []byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, nil, err
	}
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, nil, nil
	}
	data, err := readLimited(resp, limit)
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, data, nil
}

// readLimited reads resp's body, failing with a *ResponseTooLargeError
// beyond limit bytes, whether or not the server declared the length.
func readLimited(resp *http.Response, limit int64) ([]byte, error) {
	if resp.ContentLength > limit {
		return nil, &ResponseTooLargeError{Limit: limit}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, &ResponseTooLargeError{Limit: limit}
	}
	return data, nil
}

// Download downloads rawURL into memory, reading at most limit bytes.
func (c *Client) Download(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	status, data, err := c.GetLimited(ctx, rawURL, limit)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("download returned status %d", status)
	}
	return data, nil
}

// VerifyChecksum downloads the checksums file at checksumsURL (at most
// MaxChecksumsBytes) and checks that data matches the entry for assetName.
func (c *Client) VerifyChecksum(ctx context.Context, data []byte, assetName, checksumsURL string) error {
	status, checksums, err := c.GetLimited(ctx, checksumsURL, MaxChecksumsBytes)
	if err != nil {
		return fmt.Errorf("downloading checksums: %w", err)
	}
	if status != http.StatusOK {
		return fmt.Errorf("checksums download returned status %d", status)
	}
	return VerifySHA256(data, checksums, assetName)
}

// LookupChecksum returns the hex sha256 recorded for assetName in a
// checksums file in the sha256sum / GoReleaser format ("<hash>  <name>" per
// line; a "*" before the name marks binary mode and is ignored).
func LookupChecksum(checksums []byte, assetName string) (string, bool) {
	scanner := bufio.NewScanner(bytes.NewReader(checksums))
	scanner.Buffer(nil, MaxChecksumsBytes)
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == assetName {
			return fields[0], true
		}
	}
	return "", false
}

// VerifySHA256 checks that data matches the entry for assetName in a
// checksums file (see LookupChecksum).
func VerifySHA256(data, checksums []byte, assetName string) error {
	expected, ok := LookupChecksum(checksums, assetName)
	if !ok {
		return fmt.Errorf("no checksum found for %s", assetName)
	}
	actual := SHA256Hex(data)
	if !strings.EqualFold(actual, expected) {
		return fmt.Errorf("expected %s, got %s", expected, actual)
	}
	return nil
}

// SHA256Hex returns the lowercase hex sha256 digest of data.
func SHA256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
