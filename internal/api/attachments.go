package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// AttachmentService handles attachment metadata, upload, and download API
// calls.
type AttachmentService struct {
	client *Client
}

// Get retrieves the metadata for a single attachment by ID via
// GET /attachments/:id.json (filename, content type, size, author,
// content_url, ...).
func (s *AttachmentService) Get(ctx context.Context, id int) (*models.Attachment, error) {
	var resp struct {
		Attachment models.Attachment `json:"attachment"`
	}
	if err := s.client.Get(ctx, fmt.Sprintf("/attachments/%d.json", id), nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Attachment, nil
}

// Download streams the raw bytes of an attachment to w without buffering the
// whole file in memory. It returns the number of bytes written.
//
// The download URL is resolved from the attachment's content_url when that URL
// is hosted on the configured Redmine server (so subdirectory installs keep
// working), and otherwise falls back to the canonical
// /attachments/download/:id/:filename path built from the base URL (see
// downloadURL for unsafe filenames). This guard
// ensures the API key (added to every request by authTransport) is never sent
// to a host other than the configured server.
//
// Redirects are restricted by downloadRedirectPolicy: they may stay on the
// configured server, or go to a public https destination (object storage)
// without credentials; anything else is refused. The body is streamed and not
// size-limited.
func (s *AttachmentService) Download(ctx context.Context, att *models.Attachment, w io.Writer) (int64, error) {
	u := s.downloadURL(att)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return 0, err
	}

	start := time.Now()
	resp, err := s.downloadHTTPClient().Do(req)
	duration := time.Since(start)
	if err != nil {
		err = scrubRequestError(err)
		s.client.debugLog.Printf("HTTP %s %s -> error (%s): %v", req.Method, debug.ScrubURL(req.URL.String()), duration.Round(time.Millisecond), err)
		return 0, fmt.Errorf("download request failed: %w", err)
	}
	defer resp.Body.Close()

	s.client.debugLog.Printf("HTTP %s %s -> %d (%s)", req.Method, debug.ScrubURL(req.URL.String()), resp.StatusCode, duration.Round(time.Millisecond))

	if resp.StatusCode >= 400 {
		return 0, parseErrorResponse(resp)
	}

	n, err := io.Copy(w, resp.Body)
	if err != nil {
		return n, fmt.Errorf("streaming attachment %d: %w", att.ID, err)
	}
	return n, nil
}

// downloadHTTPClient returns the client used for attachment downloads. Clients
// built by NewClient have a dedicated one; otherwise the regular client is
// reused with the download redirect policy applied.
func (s *AttachmentService) downloadHTTPClient() *http.Client {
	if s.client.downloadClient != nil {
		return s.client.downloadClient
	}
	server, err := url.Parse(s.client.baseURL)
	if err != nil {
		server = &url.URL{}
	}
	hc := *s.client.httpClient
	hc.CheckRedirect = downloadRedirectPolicy(server)
	return &hc
}

// downloadURL returns the absolute URL to stream the attachment bytes from.
// See Download for the host-matching rationale. The server-supplied filename
// only decorates the built path, so one that is not a safe single path
// segment (empty, a dot segment, or containing separators) is dropped:
// Redmine serves /attachments/download/:id without a filename as well.
func (s *AttachmentService) downloadURL(att *models.Attachment) string {
	if att.ContentURL != "" && strings.HasPrefix(att.ContentURL, s.client.baseURL+"/") {
		return att.ContentURL
	}
	u := s.client.baseURL + "/attachments/download/" + strconv.Itoa(att.ID)
	if name, err := pathSegment("attachment filename", att.Filename); err == nil {
		u += "/" + name
	}
	return u
}

// Upload streams body (of known size) to Redmine's /uploads.json endpoint and
// returns the upload token. filename is sent as a query parameter per the
// Redmine REST docs so the server records the original name.
func (s *AttachmentService) Upload(ctx context.Context, filename string, body io.Reader, size int64) (string, error) {
	u := s.client.baseURL + "/uploads.json"
	if filename != "" {
		u += "?filename=" + url.QueryEscape(filename)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	if size >= 0 {
		req.ContentLength = size
	}

	start := time.Now()
	resp, err := s.client.httpClient.Do(req)
	duration := time.Since(start)
	if err != nil {
		err = scrubRequestError(err)
		s.client.debugLog.Printf("HTTP %s %s -> error (%s): %v", req.Method, debug.ScrubURL(req.URL.String()), duration.Round(time.Millisecond), err)
		return "", fmt.Errorf("upload request failed: %w", err)
	}
	defer resp.Body.Close()

	s.client.debugLog.Printf("HTTP %s %s -> %d (%s)", req.Method, debug.ScrubURL(req.URL.String()), resp.StatusCode, duration.Round(time.Millisecond))

	if resp.StatusCode >= 400 {
		return "", parseErrorResponse(resp)
	}

	var out struct {
		Upload struct {
			Token string `json:"token"`
		} `json:"upload"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxJSONResponseBytes)).Decode(&out); err != nil {
		return "", fmt.Errorf("decoding upload response: %w", err)
	}
	if out.Upload.Token == "" {
		return "", fmt.Errorf("upload response missing token")
	}
	return out.Upload.Token, nil
}
