package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/aarondpn/redmine-cli/internal/debug"
)

// AttachmentService handles file upload API calls.
type AttachmentService struct {
	client *Client
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
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", fmt.Errorf("decoding upload response: %w", err)
	}
	if out.Upload.Token == "" {
		return "", fmt.Errorf("upload response missing token")
	}
	return out.Upload.Token, nil
}
