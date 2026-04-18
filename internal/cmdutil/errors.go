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

// FormatError converts an API error into a user-friendly message.
func FormatError(err error) string {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return err.Error()
	}

	switch {
	case apiErr.IsAuthError():
		return "Authentication failed. Run 'redmine auth login' to reconfigure your credentials."
	case apiErr.IsForbidden():
		return "Permission denied: you don't have the required permissions for this action. Check your Redmine role permissions or contact your administrator."
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
	case apiErr.StatusCode >= 500:
		return fmt.Sprintf("Redmine server error (%d). Please try again later.", apiErr.StatusCode)
	default:
		return apiErr.Error()
	}
}

// BuildErrorEnvelope produces a structured error payload suitable for JSON
// rendering. It reuses FormatError for the human-readable message and derives
// a stable code from the underlying api.APIError classification.
func BuildErrorEnvelope(err error) output.ErrorEnvelope {
	env := output.ErrorEnvelope{
		Error: output.ErrorDetail{
			Message: FormatError(err),
			Code:    output.ErrCodeUnknown,
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
		case apiErr.StatusCode >= 500:
			env.Error.Code = output.ErrCodeServerError
		}
	}

	return env
}
