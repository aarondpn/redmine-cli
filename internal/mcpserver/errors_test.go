package mcpserver

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
)

func TestDescribeAPIError(t *testing.T) {
	cases := []struct {
		name      string
		err       error
		wantParts []string
	}{
		{
			name:      "auth",
			err:       &api.APIError{StatusCode: http.StatusUnauthorized, URL: "/issues.json"},
			wantParts: []string{"Authentication", "--profile"},
		},
		{
			name:      "forbidden",
			err:       &api.APIError{StatusCode: http.StatusForbidden},
			wantParts: []string{"Forbidden"},
		},
		{
			name:      "not found",
			err:       &api.APIError{StatusCode: http.StatusNotFound, URL: "/issues/99.json"},
			wantParts: []string{"not found", "/issues/99.json"},
		},
		{
			name:      "validation",
			err:       &api.APIError{StatusCode: http.StatusUnprocessableEntity, Errors: []string{"Subject can't be blank"}},
			wantParts: []string{"Validation error", "Subject can't be blank"},
		},
		{
			name:      "other api",
			err:       &api.APIError{StatusCode: http.StatusInternalServerError, Errors: []string{"boom"}},
			wantParts: []string{"500", "boom"},
		},
		{
			name:      "plain error",
			err:       errors.New("network dropped"),
			wantParts: []string{"network dropped"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := describeAPIError(tc.err)
			for _, p := range tc.wantParts {
				if !strings.Contains(got, p) {
					t.Errorf("message %q missing %q", got, p)
				}
			}
		})
	}
}

func TestToolErrSetsIsError(t *testing.T) {
	res, v, err := toolErr[any]("something went wrong")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != nil {
		t.Errorf("expected zero value, got %v", v)
	}
	if res == nil || !res.IsError {
		t.Fatal("expected IsError=true result")
	}
	if len(res.Content) == 0 {
		t.Fatal("expected at least one content block")
	}
}
