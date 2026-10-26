package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/textsafe"
)

// APIError represents an error response from the Redmine API.
type APIError struct {
	StatusCode int
	Errors     []string
	URL        string
}

// NewAPIError builds the APIError for a response with the given status from
// the messages in its body. Server-provided messages are sanitized for the
// terminal (see textsafe.Terminal) and rawURL is scrubbed of credentials.
// parseErrorResponse uses it, and so should any caller that builds an
// APIError from a raw response (such as `redmine api`).
func NewAPIError(statusCode int, rawURL string, messages []string) *APIError {
	apiErr := &APIError{StatusCode: statusCode, URL: debug.ScrubURL(rawURL)}
	for _, msg := range messages {
		apiErr.Errors = append(apiErr.Errors, textsafe.Terminal(msg))
	}
	return apiErr
}

// Error formats the error. The URL is scrubbed again here because an
// APIError may also be built as a literal, without NewAPIError.
func (e *APIError) Error() string {
	if len(e.Errors) > 0 {
		return fmt.Sprintf("API error %d: %s", e.StatusCode, strings.Join(e.Errors, "; "))
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

// parseErrorResponse extracts error messages from a Redmine error response
// into an APIError (see NewAPIError). The body is read up to
// maxErrorBodyBytes; anything beyond is discarded rather than failing.
func parseErrorResponse(resp *http.Response) *APIError {
	var rawURL string
	if resp.Request != nil && resp.Request.URL != nil {
		rawURL = resp.Request.URL.String()
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodyBytes+1))
	if err != nil {
		return NewAPIError(resp.StatusCode, rawURL, nil)
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
	return NewAPIError(resp.StatusCode, rawURL, messages)
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
