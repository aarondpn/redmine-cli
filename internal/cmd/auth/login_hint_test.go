package auth

import (
	"errors"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

func TestLoginConnectError_ForbiddenGetsProxyHint(t *testing.T) {
	err := loginConnectError(&api.APIError{StatusCode: 403})

	env := cmdutil.BuildErrorEnvelope(err)
	if env.Error.Code != "forbidden" {
		t.Errorf("code = %q, want forbidden", env.Error.Code)
	}
	if env.Error.Hint != loginProxyHint {
		t.Errorf("hint = %q, want the login proxy hint", env.Error.Hint)
	}

	text := cmdutil.FormatError(err)
	if strings.Count(text, "Hint:") != 1 || !strings.Contains(text, "--header") {
		t.Errorf("text should carry exactly the proxy hint: %q", text)
	}
	if strings.Contains(text, "project roles") {
		t.Errorf("generic forbidden hint should be replaced: %q", text)
	}
}

func TestLoginConnectError_OtherErrorsUnchanged(t *testing.T) {
	err := loginConnectError(errors.New("dial tcp: connection refused"))
	if hint := cmdutil.ErrorHint(err); hint != "" {
		t.Errorf("hint = %q, want none", hint)
	}
	if !strings.Contains(err.Error(), "could not connect to Redmine server") {
		t.Errorf("err = %v", err)
	}
}
