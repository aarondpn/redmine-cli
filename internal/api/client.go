package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aarondpn/redmine-cli/internal/config"
	"github.com/aarondpn/redmine-cli/internal/debug"
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
	httpClient *http.Client
	baseURL    string
	debugLog   *debug.Logger

	Issues       *IssueService
	Projects     *ProjectService
	TimeEntries  *TimeEntryService
	Users        *UserService
	Trackers     *TrackerService
	Statuses     *StatusService
	Enumerations *EnumerationService
	Versions     *VersionService
	Categories   *CategoryService
	Groups       *GroupService
	Search       *SearchService
	Memberships  *MembershipService
	Attachments  *AttachmentService
}

// DebugLog returns the client's debug logger.
func (c *Client) DebugLog() *debug.Logger {
	return c.debugLog
}

// authTransport applies authentication headers to every request.
type authTransport struct {
	base       http.RoundTripper
	authMethod string
	apiKey     string
	username   string
	password   string
}

func (t *authTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	switch t.authMethod {
	case "basic":
		req.SetBasicAuth(t.username, t.password)
	default:
		req.Header.Set("X-Redmine-API-Key", t.apiKey)
	}

	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}

// NewClient creates a new Redmine API client from configuration.
func NewClient(cfg *config.Config, log *debug.Logger) (*Client, error) {
	if cfg.Server == "" {
		return nil, fmt.Errorf("server URL not configured. Run 'redmine auth login' to set up")
	}

	baseURL := strings.TrimRight(cfg.Server, "/")

	transport := &authTransport{
		base:       http.DefaultTransport,
		authMethod: cfg.AuthMethod,
		apiKey:     cfg.APIKey,
		username:   cfg.Username,
		password:   cfg.Password,
	}

	c := &Client{
		httpClient: &http.Client{Transport: transport},
		baseURL:    baseURL,
		debugLog:   log,
	}

	c.Issues = &IssueService{client: c}
	c.Projects = &ProjectService{client: c}
	c.TimeEntries = &TimeEntryService{client: c}
	c.Users = &UserService{client: c}
	c.Trackers = &TrackerService{client: c}
	c.Statuses = &StatusService{client: c}
	c.Enumerations = &EnumerationService{client: c}
	c.Versions = &VersionService{client: c}
	c.Categories = &CategoryService{client: c}
	c.Groups = &GroupService{client: c}
	c.Search = &SearchService{client: c}
	c.Memberships = &MembershipService{client: c}
	c.Attachments = &AttachmentService{client: c}

	return c, nil
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
		c.debugLog.Printf("HTTP %s %s -> error (%s): %v", req.Method, debug.ScrubURL(req.URL.String()), duration.Round(time.Millisecond), err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	c.debugLog.Printf("HTTP %s %s -> %d (%s)", req.Method, debug.ScrubURL(req.URL.String()), resp.StatusCode, duration.Round(time.Millisecond))

	respBody, err := io.ReadAll(resp.Body)
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
		c.debugLog.Printf("HTTP %s %s -> error (%s): %v", req.Method, debug.ScrubURL(req.URL.String()), duration.Round(time.Millisecond), err)
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	c.debugLog.Printf("HTTP %s %s -> %d (%s)", req.Method, debug.ScrubURL(req.URL.String()), resp.StatusCode, duration.Round(time.Millisecond))

	if resp.StatusCode >= 400 {
		return parseErrorResponse(resp)
	}

	if out != nil && resp.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("reading response: %w", err)
		}
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
	}

	return nil
}
