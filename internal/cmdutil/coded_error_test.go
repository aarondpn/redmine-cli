package cmdutil

import (
	"errors"
	"fmt"
	"testing"
)

func TestCodedError_RenderedByErrorPath(t *testing.T) {
	cause := errors.New("cause")
	err := fmt.Errorf("wrapped: %w", &CodedError{
		Code:    "status_not_applied",
		Message: "Issue #1: status was not changed",
		Details: []string{"Allowed statuses: New (1).", "Other changes may already have been applied."},
		Err:     cause,
	})

	env := BuildErrorEnvelope(err)
	if env.Error.Code != "status_not_applied" || env.Error.Message != "Issue #1: status was not changed" {
		t.Errorf("envelope = %+v", env)
	}
	if len(env.Error.Details) != 2 {
		t.Errorf("details = %q", env.Error.Details)
	}

	var coded *CodedError
	if !errors.As(err, &coded) || coded.Error() != "Issue #1: status was not changed\n  - Allowed statuses: New (1).\n  - Other changes may already have been applied." {
		t.Errorf("Error() = %q", coded.Error())
	}
	if !errors.Is(err, cause) {
		t.Error("CodedError should unwrap to its cause")
	}
}
