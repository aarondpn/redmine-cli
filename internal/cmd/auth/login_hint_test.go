package auth

import (
	"errors"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

func TestLoginConnectError_ForbiddenGetsLoginHint(t *testing.T) {
	err := loginConnectError(&api.APIError{StatusCode: 403}, "apikey", false)

	env := cmdutil.BuildErrorEnvelope(err)
	if env.Error.Code != "forbidden" {
		t.Errorf("code = %q, want forbidden", env.Error.Code)
	}
	if env.Error.Hint != loginForbiddenHint {
		t.Errorf("hint = %q, want the login forbidden hint", env.Error.Hint)
	}

	text := cmdutil.FormatError(err)
	if strings.Count(text, "Hint:") != 1 || !strings.Contains(text, "--header") || !strings.Contains(text, "Enable REST web service") {
		t.Errorf("text should carry exactly the login forbidden hint: %q", text)
	}
	if strings.Contains(text, "project roles") {
		t.Errorf("generic forbidden hint should be replaced: %q", text)
	}
}

func TestLoginConnectError_UnauthorizedGetsLoginHint(t *testing.T) {
	err := loginConnectError(&api.APIError{StatusCode: 401}, "apikey", false)

	if hint := cmdutil.ErrorHint(err); hint != loginUnauthorizedAPIKeyHint {
		t.Errorf("hint = %q, want the login unauthorized hint", hint)
	}
	if hint := cmdutil.ErrorHint(loginConnectError(&api.APIError{StatusCode: 401}, "basic", false)); hint != loginUnauthorizedBasicHint {
		t.Errorf("basic hint = %q", hint)
	}
	if text := cmdutil.FormatError(err); strings.Count(text, "Hint:") != 1 || strings.Contains(text, "auth status") {
		t.Errorf("text should carry exactly the login unauthorized hint: %q", text)
	}
}

func TestLoginConnectError_Hints(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{name: "not found", err: &api.APIError{StatusCode: 404}, want: loginNotFoundHint},
		{name: "unreachable", err: errors.New("dial tcp: connection refused"), want: loginUnreachableHint},
		{name: "server error", err: &api.APIError{StatusCode: 500}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loginConnectError(tt.err, "apikey", false)
			if hint := cmdutil.ErrorHint(err); hint != tt.want {
				t.Errorf("hint = %q, want %q", hint, tt.want)
			}
			if !strings.Contains(err.Error(), "could not connect to Redmine server") {
				t.Errorf("err = %v", err)
			}
		})
	}
}

func TestLoginConnectError_AssumedHTTPSHint(t *testing.T) {
	err := loginConnectError(errors.New("dial tcp: connection refused"), "apikey", true)
	if hint := cmdutil.ErrorHint(err); hint != loginUnreachableHint+loginHTTPSAssumedHint {
		t.Errorf("hint = %q", hint)
	}
}
