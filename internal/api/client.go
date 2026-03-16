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

	"github.com/aarondpn/redmine-cli/internal/config"
)

// Client is the Redmine API client.
type Client struct {
	httpClient *http.Client
	baseURL    string

	Issues       *IssueService
	Projects     *ProjectService
	TimeEntries  *TimeEntryService
	Users        *UserService
	Trackers     *TrackerService
	Statuses     *StatusService
	Enumerations *EnumerationService
	Versions     *VersionService
	Groups       *GroupService
	Search       *SearchService
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
	req.Header.Set("Content-Type", "application/json")

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
func NewClient(cfg *config.Config) (*Client, error) {
	if cfg.Server == "" {
		return nil, fmt.Errorf("server URL not configured. Run 'redmine init' to set up")
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
	}

	c.Issues = &IssueService{client: c}
	c.Projects = &ProjectService{client: c}
	c.TimeEntries = &TimeEntryService{client: c}
	c.Users = &UserService{client: c}
	c.Trackers = &TrackerService{client: c}
	c.Statuses = &StatusService{client: c}
	c.Enumerations = &EnumerationService{client: c}
	c.Versions = &VersionService{client: c}
	c.Groups = &GroupService{client: c}
	c.Search = &SearchService{client: c}

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

func (c *Client) do(req *http.Request, out interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

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
