package cmdutil

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// SilentError is returned when the error message has already been printed
// or should be suppressed. main.go will still exit with the given code.
type SilentError struct{ Code int }

func (e *SilentError) Error() string { return "" }

// HintError decorates an error with a short, actionable next step (e.g. the
// command that lists valid values). FormatError renders the hint on its own
// "Hint:" line and BuildErrorEnvelope exposes it as error.hint, so both human
// and JSON consumers learn how to recover without parsing the message.
type HintError struct {
	Err  error
	Hint string
}

func (e *HintError) Error() string { return e.Err.Error() }

func (e *HintError) Unwrap() error { return e.Err }

// WithHint attaches hint to err. It returns err unchanged when err is nil or
// hint is empty.
func WithHint(err error, hint string) error {
	if err == nil || hint == "" {
		return err
	}
	return &HintError{Err: err, Hint: hint}
}

// Hints for authentication-related responses. Redmine answers these
// statuses with an empty body, so the status (and for 403 the endpoint) is all
// there is to go on:
//
//   - 401 on a login-required endpoint: Redmine did not recognise the
//     credentials (invalid, revoked or missing key, or a locked account) and
//     treats the request as anonymous. Endpoints that are public still answer
//     200 with public data in that case.
//   - 403 on /users/current.json or /my/account.json: any logged-in user may
//     read them, so the credentials are not the problem. With the REST API
//     disabled Redmine answers 403 to every request that needs a login, valid
//     key or not. A proxy or firewall in front of Redmine is the other cause.
//   - 403 elsewhere is ambiguous: a genuine missing permission looks the same
//     as the REST API being disabled.
//
// Agents follow hints literally, so "don't retry with other credentials" is
// only given where a missing permission is the likely cause.
const (
	unauthorizedHint = "Redmine did not accept the credentials: the API key is invalid, revoked or missing, or the account is locked. Run 'redmine auth status' to check the active profile, then 'redmine auth login' to configure a valid API key (My account > API access key)."

	restDisabledHint = "Redmine refused a request every logged-in user may make, so the credentials are most likely not the cause. The REST API is probably disabled (an administrator must enable Administration > Settings > API > Enable REST web service), or a proxy or firewall in front of Redmine blocks the request. Run 'redmine auth status' to check."

	forbiddenHint = "A 403 is either a missing permission or a disabled REST API. If other commands work for this profile, the user lacks the permission (access comes from project roles, or admin status for admin-only endpoints): don't retry with other credentials; ask the user or a Redmine administrator. If every command is denied, run 'redmine auth status'; an administrator may need to enable Administration > Settings > API > Enable REST web service."
)

// ErrorHint returns the actionable hint for err, or "" when there is none.
// An explicit hint attached via WithHint wins over generic, status-based
// hints.
func ErrorHint(err error) string {
	var hinted *HintError
	if errors.As(err, &hinted) && hinted.Hint != "" {
		return hinted.Hint
	}
	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.IsAuthError():
			return unauthorizedHint
		case apiErr.IsForbidden() && isIdentityEndpoint(apiErr.URL):
			return restDisabledHint
		case apiErr.IsForbidden():
			return forbiddenHint
		}
	}
	return ""
}

// isIdentityEndpoint reports whether rawURL is one of the endpoints that
// every logged-in user may read, regardless of roles.
func isIdentityEndpoint(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	return strings.HasSuffix(u.Path, "/users/current.json") || strings.HasSuffix(u.Path, "/my/account.json")
}

// FormatError converts an API error into a user-friendly message, followed by
// a "Hint:" line when ErrorHint has a next step for it.
func FormatError(err error) string {
	msg := FormatErrorMessage(err)
	if hint := ErrorHint(err); hint != "" {
		msg += "\nHint: " + hint
	}
	return msg
}

// FormatErrorMessage renders the human-readable description of err without
// any hint. Use it when the error is printed inline and also returned, so the
// hint is shown once by the top-level error path.
func FormatErrorMessage(err error) string {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return err.Error()
	}

	switch {
	case apiErr.IsAuthError():
		return "Authentication failed: Redmine did not accept the credentials."
	case apiErr.IsForbidden() && isIdentityEndpoint(apiErr.URL):
		return "Access denied: Redmine refused the request for the current user (is the REST API enabled?)."
	case apiErr.IsForbidden():
		return "Access denied: this is either a missing permission or a disabled REST API."
	case apiErr.IsNotFound():
		return "Resource not found."
	case apiErr.IsValidationError():
		msg := "Validation error"
		if len(apiErr.Errors) > 0 {
			msg += ":"
			for _, e := range apiErr.Errors {
				msg += fmt.Sprintf("\n  - %s", e)
			}
		}
		return msg
	case apiErr.IsConflict():
		msg := "Conflict: the resource was modified since it was last fetched. Refetch and retry."
		if len(apiErr.Errors) > 0 {
			for _, e := range apiErr.Errors {
				msg += fmt.Sprintf("\n  - %s", e)
			}
		}
		return msg
	case apiErr.StatusCode >= 500:
		return fmt.Sprintf("Redmine server error (%d). Please try again later.", apiErr.StatusCode)
	default:
		return apiErr.Error()
	}
}

// BuildErrorEnvelope produces a structured error payload suitable for JSON
// rendering. It reuses FormatError's human-readable message (without the hint
// line), derives a stable code from the underlying api.APIError
// classification, and carries ErrorHint in the separate hint field.
func BuildErrorEnvelope(err error) output.ErrorEnvelope {
	var coded *CodedError
	if errors.As(err, &coded) {
		return output.ErrorEnvelope{Error: output.ErrorDetail{
			Message: coded.Message,
			Code:    coded.Code,
			Details: append([]string(nil), coded.Details...),
			Hint:    ErrorHint(err),
		}}
	}

	env := output.ErrorEnvelope{
		Error: output.ErrorDetail{
			Message: FormatErrorMessage(err),
			Code:    output.ErrCodeUnknown,
			Hint:    ErrorHint(err),
		},
	}

	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		switch {
		case apiErr.IsAuthError():
			env.Error.Code = output.ErrCodeAuthFailed
		case apiErr.IsForbidden():
			env.Error.Code = output.ErrCodeForbidden
		case apiErr.IsNotFound():
			env.Error.Code = output.ErrCodeNotFound
		case apiErr.IsValidationError():
			env.Error.Code = output.ErrCodeValidationFailed
			if len(apiErr.Errors) > 0 {
				env.Error.Details = append([]string(nil), apiErr.Errors...)
			}
		case apiErr.IsConflict():
			env.Error.Code = output.ErrCodeConflict
			if len(apiErr.Errors) > 0 {
				env.Error.Details = append([]string(nil), apiErr.Errors...)
			}
		case apiErr.StatusCode >= 500:
			env.Error.Code = output.ErrCodeServerError
		}
	}
	// Redmine answers 200 but withholds the data; treat it as a permission
	// failure so callers can branch on the code.
	if errors.Is(err, api.ErrWatchersNotVisible) {
		env.Error.Code = output.ErrCodeForbidden
	}
	// An identifier that cannot be used as a URL path segment is rejected
	// before any request is sent; it is the caller's input that is invalid.
	var pathErr *api.InvalidPathSegmentError
	if errors.As(err, &pathErr) {
		env.Error.Code = output.ErrCodeValidationFailed
	}
	// Failures that never got an HTTP status.
	var urlErr *url.Error
	if errors.As(err, &urlErr) && urlErr.Timeout() {
		env.Error.Code = output.ErrCodeTimeout
	}
	var existsErr *DestinationExistsError
	if errors.As(err, &existsErr) {
		env.Error.Code = output.ErrCodeAlreadyExists
	}

	return env
}
