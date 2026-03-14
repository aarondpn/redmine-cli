package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// APIError represents an error response from the Redmine API.
type APIError struct {
	StatusCode int
	Errors     []string
	URL        string
}

func (e *APIError) Error() string {
	if len(e.Errors) > 0 {
		return fmt.Sprintf("API error %d: %s", e.StatusCode, strings.Join(e.Errors, "; "))
	}
	return fmt.Sprintf("API error %d for %s", e.StatusCode, e.URL)
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

// parseErrorResponse extracts error messages from a Redmine error response.
func parseErrorResponse(resp *http.Response) *APIError {
	apiErr := &APIError{
		StatusCode: resp.StatusCode,
		URL:        resp.Request.URL.String(),
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return apiErr
	}

	var errResp struct {
		Errors []string `json:"errors"`
	}
	if json.Unmarshal(body, &errResp) == nil && len(errResp.Errors) > 0 {
		apiErr.Errors = errResp.Errors
	}

	return apiErr
}
