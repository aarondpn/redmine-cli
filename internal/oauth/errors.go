package oauth

import (
	"errors"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"

	"github.com/aarondpn/redmine-cli/v2/internal/textsafe"
)

var (
	ErrUnsupported   = errors.New("OAuth requires Redmine 6.1 or newer with the REST API enabled")
	ErrInvalidClient = errors.New("Redmine rejected the OAuth client ID or secret")
	ErrInvalidScope  = errors.New("Redmine rejected the requested OAuth scopes")
	ErrAccessDenied  = errors.New("the login was denied in the browser")
	ErrLoginRequired = errors.New("the OAuth login expired or was revoked")
)

// detailError carries a sanitized message and keeps the original error
// reachable for errors.As.
type detailError struct {
	msg string
	err error
}

func (e *detailError) Error() string { return e.msg }
func (e *detailError) Unwrap() error { return e.err }

// tokenError maps a failed token request to one of the Err values or a
// sanitized error. refresh tells a refresh_token grant from a code exchange.
// The server's response body is never included as is.
func tokenError(err error, refresh bool) error {
	var re *oauth2.RetrieveError
	if !errors.As(err, &re) {
		return fmt.Errorf("OAuth token request failed: %w", err)
	}
	status := 0
	if re.Response != nil {
		status = re.Response.StatusCode
	}
	switch {
	case status == http.StatusNotFound:
		return fmt.Errorf("%w", ErrUnsupported)
	case re.ErrorCode == "invalid_client" || status == http.StatusUnauthorized:
		return fmt.Errorf("%w", ErrInvalidClient)
	case re.ErrorCode == "invalid_scope":
		return fmt.Errorf("%w", ErrInvalidScope)
	case re.ErrorCode == "invalid_grant" && refresh:
		return fmt.Errorf("%w", ErrLoginRequired)
	case re.ErrorCode == "invalid_grant":
		return &detailError{"Redmine rejected the authorization code" + describe(re), err}
	case re.ErrorCode != "":
		return &detailError{"OAuth token request failed: " + textsafe.Terminal(re.ErrorCode) + describe(re), err}
	}
	return &detailError{fmt.Sprintf("OAuth token request failed: HTTP %d", status), err}
}

// describe formats the optional error_description as ": text".
func describe(re *oauth2.RetrieveError) string {
	if re.ErrorDescription == "" {
		return ""
	}
	return ": " + textsafe.Terminal(re.ErrorDescription)
}
