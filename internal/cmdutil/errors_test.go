package cmdutil

import (
	"errors"
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
