package cmdutil

import (
	"errors"
	"fmt"

	"github.com/aarondpn/redmine-cli/internal/api"
)

// FormatError converts an API error into a user-friendly message.
func FormatError(err error) string {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return err.Error()
	}

	switch {
	case apiErr.IsAuthError():
		return "Authentication failed. Run 'redmine init' to reconfigure your credentials."
	case apiErr.IsForbidden():
		return "Permission denied. You may not have access to this resource."
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
	default:
		return apiErr.Error()
	}
}
