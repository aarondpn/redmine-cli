package cmdutil

import (
	"errors"
	"fmt"

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

// forbiddenHint is the generic next step for 403 responses. Agents in
// particular tend to retry with another profile; that is rarely what the
// user wants and does not fix a missing permission.
const forbiddenHint = "Redmine denied this for the current user; access comes from project roles, or admin status for admin-only endpoints. Don't retry with other credentials; ask the user or a Redmine administrator."

// ErrorHint returns the actionable hint for err, or "" when there is none.
// An explicit hint attached via WithHint wins over generic, status-based
// hints.
func ErrorHint(err error) string {
	var hinted *HintError
	if errors.As(err, &hinted) && hinted.Hint != "" {
		return hinted.Hint
	}
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && apiErr.IsForbidden() {
		return forbiddenHint
	}
	return ""
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
		return "Authentication failed. Run 'redmine auth login' to reconfigure your credentials."
	case apiErr.IsForbidden():
		return "Permission denied: you don't have the required permissions for this action."
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

	return env
}
