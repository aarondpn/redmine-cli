package cmdutil

import (
	"errors"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func TestBuildErrorEnvelope_NotFound(t *testing.T) {
	env := BuildErrorEnvelope(&api.APIError{StatusCode: 404})
	if env.Error.Code != output.ErrCodeNotFound {
		t.Errorf("code = %q, want %q", env.Error.Code, output.ErrCodeNotFound)
	}
}

func TestBuildErrorEnvelope_Auth(t *testing.T) {
	env := BuildErrorEnvelope(&api.APIError{StatusCode: 401})
	if env.Error.Code != output.ErrCodeAuthFailed {
		t.Errorf("code = %q, want %q", env.Error.Code, output.ErrCodeAuthFailed)
	}
}

func TestBuildErrorEnvelope_Forbidden(t *testing.T) {
	env := BuildErrorEnvelope(&api.APIError{StatusCode: 403})
	if env.Error.Code != output.ErrCodeForbidden {
		t.Errorf("code = %q, want %q", env.Error.Code, output.ErrCodeForbidden)
	}
}

func TestBuildErrorEnvelope_ValidationIncludesDetails(t *testing.T) {
	apiErr := &api.APIError{StatusCode: 422, Errors: []string{"name is required", "email is invalid"}}
	env := BuildErrorEnvelope(apiErr)
	if env.Error.Code != output.ErrCodeValidationFailed {
		t.Errorf("code = %q, want %q", env.Error.Code, output.ErrCodeValidationFailed)
	}
	if len(env.Error.Details) != 2 {
		t.Fatalf("details: got %d, want 2", len(env.Error.Details))
	}
}

func TestBuildErrorEnvelope_ConflictIncludesDetails(t *testing.T) {
	apiErr := &api.APIError{StatusCode: 409, Errors: []string{"Page has been updated by someone else"}}
	env := BuildErrorEnvelope(apiErr)
	if env.Error.Code != output.ErrCodeConflict {
		t.Errorf("code = %q, want %q", env.Error.Code, output.ErrCodeConflict)
	}
	if len(env.Error.Details) != 1 || env.Error.Details[0] != "Page has been updated by someone else" {
		t.Errorf("details = %v, want server-provided message", env.Error.Details)
	}
}

func TestFormatError_Conflict(t *testing.T) {
	apiErr := &api.APIError{StatusCode: 409, Errors: []string{"Page has been updated by someone else"}}
	msg := FormatError(apiErr)
	if !strings.Contains(msg, "Conflict") {
		t.Errorf("message missing Conflict prefix: %q", msg)
	}
	if !strings.Contains(msg, "Page has been updated by someone else") {
		t.Errorf("message missing server detail: %q", msg)
	}
}

func TestBuildErrorEnvelope_ServerError(t *testing.T) {
	env := BuildErrorEnvelope(&api.APIError{StatusCode: 503})
	if env.Error.Code != output.ErrCodeServerError {
		t.Errorf("code = %q, want %q", env.Error.Code, output.ErrCodeServerError)
	}
}

func TestBuildErrorEnvelope_UnknownForGenericError(t *testing.T) {
	env := BuildErrorEnvelope(errors.New("boom"))
	if env.Error.Code != output.ErrCodeUnknown {
		t.Errorf("code = %q, want %q", env.Error.Code, output.ErrCodeUnknown)
	}
	if env.Error.Message != "boom" {
		t.Errorf("message = %q", env.Error.Message)
	}
}
