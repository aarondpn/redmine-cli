package api

import (
	"net/http"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// NewTestClient constructs a Client wired to a given http.Client and base URL
// without requiring a full config. It is exported only for use in tests of
// other packages (e.g. cmdutil) that need a working *Client.
func NewTestClient(httpClient *http.Client, baseURL string, log *debug.Logger) *Client {
	c := &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		debugLog:   log,
	}
	c.Attachments = &AttachmentService{client: c}
	c.Issues = &IssueService{client: c}
	return c
}
