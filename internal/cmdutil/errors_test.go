package cmdutil

import (
	"errors"
	"fmt"
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

func TestBuildErrorEnvelope_WatchersNotVisibleIsForbidden(t *testing.T) {
	// Commands wrap the error with context; the code must survive wrapping.
	err := fmt.Errorf("failed to list watchers for issue #1: %w", api.ErrWatchersNotVisible)
	env := BuildErrorEnvelope(err)
	if env.Error.Code != output.ErrCodeForbidden {
		t.Errorf("code = %q, want %q", env.Error.Code, output.ErrCodeForbidden)
	}
	if !strings.Contains(env.Error.Message, "View watchers permission") {
		t.Errorf("message = %q, want permission hint", env.Error.Message)
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

// A value rejected by the path-segment check is invalid input, reported
// before any request is sent.
func TestBuildErrorEnvelope_InvalidPathSegmentIsValidation(t *testing.T) {
	pathErr := &api.InvalidPathSegmentError{Kind: "project identifier", Value: "../users/2", Reason: `must not contain "/"`}
	env := BuildErrorEnvelope(fmt.Errorf("resolving project: %w", pathErr))
	if env.Error.Code != output.ErrCodeValidationFailed {
		t.Errorf("code = %q, want %q", env.Error.Code, output.ErrCodeValidationFailed)
	}
	if !strings.Contains(env.Error.Message, `invalid project identifier "../users/2"`) {
		t.Errorf("message = %q", env.Error.Message)
	}
}

func TestFormatError_HintRenderedOnOwnLine(t *testing.T) {
	err := WithHint(&api.APIError{StatusCode: 422, Errors: []string{"Category cannot be blank"}}, "Look up values with 'redmine projects get demo'.")
	msg := FormatError(err)
	want := "Validation error:\n  - Category cannot be blank\nHint: Look up values with 'redmine projects get demo'."
	if msg != want {
		t.Errorf("message = %q, want %q", msg, want)
	}
}

func TestBuildErrorEnvelope_HintIsSeparateField(t *testing.T) {
	err := WithHint(&api.APIError{StatusCode: 422, Errors: []string{"Subject cannot be blank"}}, "try this")
	env := BuildErrorEnvelope(err)
	if env.Error.Code != output.ErrCodeValidationFailed {
		t.Errorf("code = %q, want %q", env.Error.Code, output.ErrCodeValidationFailed)
	}
	if env.Error.Hint != "try this" {
		t.Errorf("hint = %q, want %q", env.Error.Hint, "try this")
	}
	if strings.Contains(env.Error.Message, "Hint:") {
		t.Errorf("message should not embed the hint: %q", env.Error.Message)
	}
	if len(env.Error.Details) != 1 {
		t.Errorf("details = %v, want the server message", env.Error.Details)
	}
}

func TestBuildErrorEnvelope_ForbiddenHasGenericHint(t *testing.T) {
	env := BuildErrorEnvelope(&api.APIError{StatusCode: 403})
	if !strings.Contains(env.Error.Hint, "Don't retry with other credentials") {
		t.Errorf("hint = %q, want forbidden guidance", env.Error.Hint)
	}
	if !strings.Contains(FormatError(&api.APIError{StatusCode: 403}), "\nHint: ") {
		t.Error("FormatError should render the forbidden hint")
	}
	if strings.Contains(FormatErrorMessage(&api.APIError{StatusCode: 403}), "Hint:") {
		t.Error("FormatErrorMessage must not include the hint")
	}
}

func TestBuildErrorEnvelope_WrappedErrorKeepsCodeAndHint(t *testing.T) {
	err := fmt.Errorf("failed to list versions: %w", &api.APIError{StatusCode: 403})
	env := BuildErrorEnvelope(err)
	if env.Error.Code != output.ErrCodeForbidden || env.Error.Hint == "" {
		t.Errorf("envelope = %+v, want forbidden code and hint", env.Error)
	}
}

func TestErrorHint_NoneForPlainErrors(t *testing.T) {
	for _, err := range []error{errors.New("boom"), &api.APIError{StatusCode: 404}, &api.APIError{StatusCode: 422}} {
		if hint := ErrorHint(err); hint != "" {
			t.Errorf("ErrorHint(%v) = %q, want empty", err, hint)
		}
		if strings.Contains(FormatError(err), "Hint:") {
			t.Errorf("FormatError(%v) unexpectedly contains a hint", err)
		}
	}
}

func TestWithHint_PassThrough(t *testing.T) {
	if WithHint(nil, "x") != nil {
		t.Error("WithHint(nil) should return nil")
	}
	base := errors.New("boom")
	if WithHint(base, "") != base {
		t.Error("WithHint with empty hint should return err unchanged")
	}
	wrapped := WithHint(base, "x")
	if !errors.Is(wrapped, base) || wrapped.Error() != "boom" {
		t.Errorf("WithHint should wrap transparently, got %v", wrapped)
	}
}
