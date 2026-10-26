package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// RawResponse holds the unprocessed HTTP response from DoRaw.
type RawResponse struct {
	StatusCode int
	Status     string
	Headers    http.Header
	Body       []byte
}

// Client is the Redmine API client.
type Client struct {
	// httpClient sends every request, API calls and attachment downloads
	// alike. Its redirect policy and dialer refuse redirects to internal
	// addresses away from the configured server (see redirectPolicy).
	httpClient *http.Client
	// dialer guards httpClient's connections; kept so tests can adjust which
	// addresses it blocks. Nil for clients built directly in tests.
	dialer   *guardedDialer
	baseURL  string
	debugLog *debug.Logger

	Issues       *IssueService
	Projects     *ProjectService
	TimeEntries  *TimeEntryService
	Users        *UserService
	MyAccount    *MyAccountService
	Trackers     *TrackerService
	Statuses     *StatusService
	Roles        *RoleService
	Enumerations *EnumerationService
	Versions     *VersionService
	Categories   *CategoryService
	Groups       *GroupService
	Search       *SearchService
	Memberships  *MembershipService
	Attachments  *AttachmentService
	Wikis        *WikiService
	Queries      *QueryService
	CustomFields *CustomFieldService
	Files        *FileService
	Relations    *RelationService
}

// DebugLog returns the client's debug logger.
func (c *Client) DebugLog() *debug.Logger {
	return c.debugLog
}

// UserAgent is the default User-Agent sent with every request. It is
// overwritten at startup with the build version; a "User-Agent" entry in the
// profile's headers takes precedence.
var UserAgent = "redmine-cli"

// Response size limits. They are variables so tests can lower them.
var (
	// maxJSONResponseBytes caps a decoded API response.
	maxJSONResponseBytes int64 = 256 << 20
	// maxRawResponseBytes caps a response returned by DoRaw.
	maxRawResponseBytes int64 = 256 << 20
	// maxErrorBodyBytes caps how much of an error response is read; the rest
	// is discarded rather than failing the request.
	maxErrorBodyBytes int64 = 1 << 20
)

// ResponseTooLargeError is returned when a response body exceeds the size
// limit for its kind. Attachment downloads are streamed and never limited.
type ResponseTooLargeError struct {
	Limit int64
}

func (e *ResponseTooLargeError) Error() string {
	if e.Limit >= 1<<20 && e.Limit%(1<<20) == 0 {
		return fmt.Sprintf("response body exceeds the %d MiB limit", e.Limit>>20)
	}
	return fmt.Sprintf("response body exceeds the %d byte limit", e.Limit)
}

// readLimited reads r fully, failing with *ResponseTooLargeError when it
// holds more than limit bytes.
func readLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, &ResponseTooLargeError{Limit: limit}
	}
	return data, nil
}

// scrubRequestError redacts credentials from the URL that net/http embeds in
// a *url.Error (for a refused redirect that is the Location header, which may
// be a presigned URL).
func scrubRequestError(err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		urlErr.URL = debug.ScrubURL(urlErr.URL)
	}
	return err
}

// authTransport applies authentication and configured headers to every request.
type authTransport struct {
	base       http.RoundTripper
	authMethod string
	apiKey     string
	username   string
	password   string
	// headers are the user-configured extra headers. Like credentials they are
	// only sent to the configured origin, since they may carry proxy tokens.
	headers map[string]string
	// origin is the configured server's origin. Credentials are only
	// attached when the request targets this exact origin, so neither a
	// redirect off-site (e.g. an attachment content_url that 302s to external
	// object storage) nor one to the same host over plain http ever receives
	// the API key or basic-auth credentials. The one exception is the https
	// upgrade of an http:// origin (see attachesTo). A nil origin means
	// "always attach" (used by tests that construct the transport directly
	// without a configured server).
	origin *config.Origin
}

// attachesTo reports whether credentials and configured headers go to u (see
// isServerOrigin).
func (t *authTransport) attachesTo(u *url.URL) bool {
	if t.origin == nil {
		return true
	}
	return isServerOrigin(*t.origin, config.OriginOf(u))
}

// isServerOrigin reports whether o is the configured server's origin, or, for
// a server configured as http://, the same host upgraded to https (port 80
// becoming 443), which is how most servers redirect plain-http clients. The
// upgrade only ever improves on the cleartext connection the user
// configured.
func isServerOrigin(server, o config.Origin) bool {
	if o == server {
		return true
	}
	if server.Scheme != "http" || o.Scheme != "https" || o.Host != server.Host {
		return false
	}
	port := server.Port
	if port == "80" {
		port = "443"
	}
	return o.Port == port
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Only attach credentials to the configured server's origin. This keeps
	// the API key (and basic-auth) from leaking to a third party, or over
	// cleartext, if the server or an attachment's content_url redirects.
	if t.attachesTo(req.URL) {
		// Headers already set on the request win over configured ones;
		// authentication below wins over both.
		for name, value := range t.headers {
			if req.Header.Get(name) == "" {
				req.Header.Set(name, value)
			}
		}
		switch t.authMethod {
		case "basic":
			req.SetBasicAuth(t.username, t.password)
		default:
			req.Header.Set("X-Redmine-API-Key", t.apiKey)
		}
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", UserAgent)
	}

	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

// NewClient creates a new Redmine API client from configuration.
//
// The transport bounds connection setup and, if asked to, the wait for
// response headers (see HTTPTimeoutEnv) but not body streaming. Redirects
// that downgrade https to http, or that leave the configured server for a
// non-public address, are refused (see redirectPolicy), and credentials and
// configured headers are only sent to the configured server's origin
// (scheme and host:port, see authTransport.attachesTo).
func NewClient(cfg *config.Config, log *debug.Logger) (*Client, error) {
	if cfg.Server == "" {
		return nil, fmt.Errorf("server URL not configured. Run 'redmine auth login' to set up")
	}
	headerTimeout, err := responseHeaderTimeout()
	if err != nil {
		return nil, err
	}
	return newClient(cfg, log, newBaseTransport(headerTimeout)), nil
}

// newClient wires a Client around base, the transport all requests share.
func newClient(cfg *config.Config, log *debug.Logger, base *http.Transport) *Client {
	baseURL := strings.TrimRight(cfg.Server, "/")

	serverURL, err := url.Parse(baseURL)
	if err != nil {
		serverURL = &url.URL{}
	}

	guarded, dialer := newGuardedTransport(base, serverURL)

	serverOrigin := config.OriginOf(serverURL)
	var transport http.RoundTripper = &authTransport{
		base:       guarded,
		authMethod: cfg.AuthMethod,
		apiKey:     cfg.APIKey,
		username:   cfg.Username,
		password:   cfg.Password,
		headers:    cfg.Headers,
		origin:     &serverOrigin,
	}
	if cfg.ReadOnly {
		transport = &readOnlyTransport{base: transport}
	}

	c := &Client{
		httpClient: &http.Client{Transport: transport, CheckRedirect: redirectPolicy(serverURL)},
		dialer:     dialer,
		baseURL:    baseURL,
		debugLog:   log,
	}

	c.Issues = &IssueService{client: c}
	c.Projects = &ProjectService{client: c}
	c.TimeEntries = &TimeEntryService{client: c}
	c.Users = &UserService{client: c}
	c.MyAccount = &MyAccountService{client: c}
	c.Trackers = &TrackerService{client: c}
	c.Statuses = &StatusService{client: c}
	c.Roles = &RoleService{client: c}
	c.Enumerations = &EnumerationService{client: c}
	c.Versions = &VersionService{client: c}
	c.Categories = &CategoryService{client: c}
	c.Groups = &GroupService{client: c}
	c.Search = &SearchService{client: c}
	c.Memberships = &MembershipService{client: c}
	c.Attachments = &AttachmentService{client: c}
	c.Wikis = &WikiService{client: c}
	c.Queries = &QueryService{client: c}
	c.CustomFields = &CustomFieldService{client: c}
	c.Files = &FileService{client: c}
	c.Relations = &RelationService{client: c}

	return c
}

// Get performs a GET request and decodes the response into out.
func (c *Client) Get(ctx context.Context, path string, params url.Values, out interface{}) error {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}

	return c.do(req, out)
}

// Post performs a POST request with a JSON body.
func (c *Client) Post(ctx context.Context, path string, body interface{}, out interface{}) error {
	return c.doWithBody(ctx, http.MethodPost, path, body, out)
}

// Put performs a PUT request with a JSON body.
func (c *Client) Put(ctx context.Context, path string, body interface{}) error {
	return c.doWithBody(ctx, http.MethodPut, path, body, nil)
}

// Delete performs a DELETE request.
func (c *Client) Delete(ctx context.Context, path string) error {
	u := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	return c.do(req, nil)
}

func (c *Client) doWithBody(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshaling request: %w", err)
		}
		bodyReader = bytes.NewReader(data)
	}

	u := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return err
	}

	return c.do(req, out)
}

// DoRaw performs an HTTP request and returns the raw response without parsing.
func (c *Client) DoRaw(ctx context.Context, method, path string, params url.Values, body io.Reader) (*RawResponse, error) {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}

	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		err = scrubRequestError(err)
		c.debugLog.Printf("HTTP %s %s -> error (%s): %v", req.Method, debug.ScrubURL(req.URL.String()), duration.Round(time.Millisecond), err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	c.debugLog.Printf("HTTP %s %s -> %d (%s)", req.Method, debug.ScrubURL(req.URL.String()), resp.StatusCode, duration.Round(time.Millisecond))

	respBody, err := readLimited(resp.Body, maxRawResponseBytes)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	return &RawResponse{
		StatusCode: resp.StatusCode,
		Status:     resp.Status,
		Headers:    resp.Header,
		Body:       respBody,
	}, nil
}

func (c *Client) do(req *http.Request, out interface{}) error {
	start := time.Now()
	resp, err := c.httpClient.Do(req)
	duration := time.Since(start)

	if err != nil {
		err = scrubRequestError(err)
		c.debugLog.Printf("HTTP %s %s -> error (%s): %v", req.Method, debug.ScrubURL(req.URL.String()), duration.Round(time.Millisecond), err)
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	c.debugLog.Printf("HTTP %s %s -> %d (%s)", req.Method, debug.ScrubURL(req.URL.String()), resp.StatusCode, duration.Round(time.Millisecond))

	if resp.StatusCode >= 400 {
		return parseErrorResponse(resp)
	}

	if out != nil && resp.StatusCode != http.StatusNoContent {
		body, err := readLimited(resp.Body, maxJSONResponseBytes)
		if err != nil {
			return fmt.Errorf("reading response: %w", err)
		}
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}
