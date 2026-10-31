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
	"github.com/aarondpn/redmine-cli/v2/internal/oauth"
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
	// oauthTokens supplies the bearer token of an OAuth profile; nil for
	// other auth methods.
	oauthTokens *oauth.TokenSource

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
	Journals     *JournalService
	News         *NewsService
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

// requestError wraps a failed request. A login that expired or a rejected
// OAuth client is returned as is: the "request failed: Get ..." prefix would
// only bury the hint.
func requestError(err error) error {
	return requestErrorf("request failed", err)
}

func requestErrorf(prefix string, err error) error {
	err = scrubRequestError(err)
	if errors.Is(err, oauth.ErrLoginRequired) || errors.Is(err, oauth.ErrInvalidClient) {
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			return urlErr.Err
		}
		return err
	}
	return fmt.Errorf("%s: %w", prefix, err)
}

// authMethodNone attaches configured headers and the User-Agent but no
// credentials (requests to the OAuth endpoints).
const authMethodNone = "none"

// authTransport applies authentication and configured headers to every request.
type authTransport struct {
	base       http.RoundTripper
	authMethod string
	apiKey     string
	username   string
	password   string
	// tokens supplies the bearer token for the oauth auth method.
	tokens *oauth.TokenSource
	// profile names the stored profile of the tokens, for the re-login hint.
	profile string
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
		case authMethodNone:
		case config.AuthMethodOAuth:
			token, err := t.tokens.Token(req.Context())
			if err != nil {
				closeBody(req)
				return nil, t.loginError(err)
			}
			req.Header.Set("Authorization", "Bearer "+token)
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
	resp, err := base.RoundTrip(req)
	if err != nil || resp.StatusCode != http.StatusUnauthorized || t.authMethod != config.AuthMethodOAuth {
		return resp, err
	}
	return t.retryUnauthorized(base, req, resp)
}

// LoginRequiredError is oauth.ErrLoginRequired for a stored profile, so the
// hint can name the profile to log in to again.
type LoginRequiredError struct {
	Profile string
	Err     error
}

func (e *LoginRequiredError) Error() string { return e.Err.Error() }
func (e *LoginRequiredError) Unwrap() error { return e.Err }

// ReloginCommand returns the command that fixes err, a failed OAuth refresh.
func ReloginCommand(err error) string {
	var lre *LoginRequiredError
	if errors.As(err, &lre) && lre.Profile != "" {
		return "redmine auth login --oauth --name " + lre.Profile
	}
	return "redmine auth login --oauth"
}

func (t *authTransport) loginError(err error) error {
	if t.profile != "" && errors.Is(err, oauth.ErrLoginRequired) {
		return &LoginRequiredError{Profile: t.profile, Err: err}
	}
	return err
}

// closeBody closes a request body that will not be sent.
func closeBody(req *http.Request) {
	if req.Body != nil {
		_ = req.Body.Close()
	}
}

// retryUnauthorized answers a 401 to a request that carried a bearer token:
// Redmine rejected the access token, so refresh it and send the request once
// more. A body that cannot be replayed gets no retry.
func (t *authTransport) retryUnauthorized(base http.RoundTripper, req *http.Request, resp *http.Response) (*http.Response, error) {
	sent, ok := strings.CutPrefix(req.Header.Get("Authorization"), "Bearer ")
	if !ok || (req.Body != nil && req.Body != http.NoBody && req.GetBody == nil) {
		return resp, nil
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, maxErrorBodyBytes))
	resp.Body.Close()

	token, err := t.tokens.Refresh(req.Context(), sent)
	if err != nil {
		return nil, t.loginError(err)
	}
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		req.Body = body
	}
	req.Header.Set("Authorization", "Bearer "+token)
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
	if cfg.AuthMethod == config.AuthMethodOAuth && cfg.OAuth.AccessToken == "" && cfg.OAuth.RefreshToken == "" {
		return nil, errors.New("no OAuth login stored. Run 'redmine auth login --oauth'")
	}
	headerTimeout, err := responseHeaderTimeout()
	if err != nil {
		return nil, err
	}
	// A scheme-less server (redmine.example.com) would fail every request
	// with 'unsupported protocol scheme ""'; assume https like the login does.
	if server := config.EnsureScheme(cfg.Server); server != cfg.Server {
		withScheme := *cfg
		withScheme.Server = server
		cfg = &withScheme
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

	var tokens *oauth.TokenSource
	var profile string
	if cfg.AuthMethod == config.AuthMethodOAuth {
		tokens = newTokenSource(cfg, log, base)
		_, profile, _ = cfg.ProfileLocation()
	}

	httpClient, dialer := newGuardedClient(base, serverURL, func(guarded http.RoundTripper) http.RoundTripper {
		serverOrigin := config.OriginOf(serverURL)
		var transport http.RoundTripper = &authTransport{
			base:       guarded,
			authMethod: cfg.AuthMethod,
			apiKey:     cfg.APIKey,
			username:   cfg.Username,
			password:   cfg.Password,
			tokens:     tokens,
			profile:    profile,
			headers:    cfg.Headers,
			origin:     &serverOrigin,
		}
		if cfg.ReadOnly {
			transport = &readOnlyTransport{base: transport}
		}
		return transport
	})

	c := &Client{
		httpClient:  httpClient,
		dialer:      dialer,
		baseURL:     baseURL,
		debugLog:    log,
		oauthTokens: tokens,
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
	c.Journals = &JournalService{client: c}
	c.News = &NewsService{client: c}
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

// newGuardedClient builds the http.Client every request of a server goes
// through: base behind the connection guard, wrapped by wrap, with the
// redirect policy.
func newGuardedClient(base *http.Transport, serverURL *url.URL, wrap func(guarded http.RoundTripper) http.RoundTripper) (*http.Client, *guardedDialer) {
	guarded, dialer := newGuardedTransport(base, serverURL)
	return &http.Client{Transport: wrap(guarded), CheckRedirect: redirectPolicy(serverURL, base.Proxy)}, dialer
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
		return nil, requestError(err)
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
		return requestError(err)
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
