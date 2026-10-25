package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// APIError represents an error response from the Redmine API.
type APIError struct {
	StatusCode int
	Errors     []string
	URL        string
}

// Error formats the error. Messages are sanitized and the URL scrubbed again
// here because an APIError may also be built outside parseErrorResponse (for
// example from a raw response by `redmine api`).
func (e *APIError) Error() string {
	if len(e.Errors) > 0 {
		return fmt.Sprintf("API error %d: %s", e.StatusCode, sanitizeServerText(strings.Join(e.Errors, "; ")))
	}
	return fmt.Sprintf("API error %d for %s", e.StatusCode, debug.ScrubURL(e.URL))
}

// IsNotFound returns true if the error is a 404.
func (e *APIError) IsNotFound() bool {
	return e.StatusCode == http.StatusNotFound
}

// IsAuthError returns true if the error is a 401.
func (e *APIError) IsAuthError() bool {
	return e.StatusCode == http.StatusUnauthorized
}

// IsForbidden returns true if the error is a 403.
func (e *APIError) IsForbidden() bool {
	return e.StatusCode == http.StatusForbidden
}

// IsValidationError returns true if the error is a 422.
func (e *APIError) IsValidationError() bool {
	return e.StatusCode == http.StatusUnprocessableEntity
}

// IsConflict returns true if the error is a 409. Redmine surfaces 409 on
// optimistic-locking failures (e.g. updating a wiki page whose version has
// been bumped on the server since the client last fetched it).
func (e *APIError) IsConflict() bool {
	return e.StatusCode == http.StatusConflict
}

// truncationMarker is appended to the error messages recovered from an error
// body that was cut off at maxErrorBodyBytes.
const truncationMarker = "…"

// parseErrorResponse extracts error messages from a Redmine error response.
// The URL is scrubbed of credentials, the body is read up to
// maxErrorBodyBytes (anything beyond is discarded rather than failing), and
// server-provided messages are stripped of terminal control characters.
func parseErrorResponse(resp *http.Response) *APIError {
	apiErr := &APIError{
		StatusCode: resp.StatusCode,
	}
	if resp.Request != nil && resp.Request.URL != nil {
		apiErr.URL = debug.ScrubURL(resp.Request.URL.String())
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes+1))
	if err != nil {
		return apiErr
	}
	truncated := int64(len(body)) > maxErrorBodyBytes
	if truncated {
		body = body[:maxErrorBodyBytes]
	}

	var errResp struct {
		Errors []string `json:"errors"`
	}
	var messages []string
	if json.Unmarshal(body, &errResp) == nil {
		messages = errResp.Errors
	} else if truncated {
		if messages = partialErrorMessages(body); len(messages) > 0 {
			messages = append(messages, truncationMarker)
		}
	}
	for _, msg := range messages {
		apiErr.Errors = append(apiErr.Errors, sanitizeServerText(msg))
	}

	return apiErr
}

// partialErrorMessages recovers the complete strings of a top-level "errors"
// array from a JSON body that was cut off mid-document.
func partialErrorMessages(body []byte) []string {
	dec := json.NewDecoder(bytes.NewReader(body))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil
	}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return nil
		}
		if key, _ := keyTok.(string); key != "errors" {
			var skip json.RawMessage
			if err := dec.Decode(&skip); err != nil {
				return nil
			}
			continue
		}
		if tok, err := dec.Token(); err != nil || tok != json.Delim('[') {
			return nil
		}
		var messages []string
		for dec.More() {
			var msg string
			if err := dec.Decode(&msg); err != nil {
				break
			}
			messages = append(messages, msg)
		}
		return messages
	}
	return nil
}

// sanitizeServerText strips control characters (C0 except newline and tab,
// DEL, and C1, which covers ESC-introduced terminal sequences) from text the
// server controls, so error messages cannot rewrite the user's terminal.
func sanitizeServerText(s string) string {
	clean := true
	for _, r := range s {
		if isStrippedControl(r) {
			clean = false
			break
		}
	}
	if clean {
		return s
	}
	return strings.Map(func(r rune) rune {
		if isStrippedControl(r) {
			return -1
		}
		return r
	}, s)
}

func isStrippedControl(r rune) bool {
	switch {
	case r == '\n' || r == '\t':
		return false
	case r < 0x20, r == 0x7f:
		return true
	case r >= 0x80 && r <= 0x9f:
		return true
	}
	return false
}
