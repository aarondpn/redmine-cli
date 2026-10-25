package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

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
			name:      "invalid path segment",
			err:       fmt.Errorf("resolving project: %w", &api.InvalidPathSegmentError{Kind: "project identifier", Value: "a/b", Reason: `must not contain "/"`}),
			wantParts: []string{"Invalid argument", `project identifier "a/b" must not contain "/"`, "not sent to Redmine"},
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

// An identifier that cannot be a URL path segment never reaches Redmine;
// the tool error says so instead of echoing the bare validation error.
func TestGetProjectTool_InvalidIdentifier(t *testing.T) {
	var paths []string
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"projects":[],"total_count":0,"offset":0,"limit":100}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_project",
		Arguments: map[string]any{"identifier": "../users/2"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("expected a tool error, got %+v", res.Content)
	}
	text := asText(t, res.Content[0])
	for _, want := range []string{"Invalid argument", `"../users/2"`, "not sent to Redmine"} {
		if !strings.Contains(text, want) {
			t.Errorf("tool error %q missing %q", text, want)
		}
	}
	for _, p := range paths {
		if strings.Contains(p, "users") {
			t.Errorf("request reached %s", p)
		}
	}
}

func TestResourceError_InvalidPathSegment(t *testing.T) {
	err := resourceError("redmine://project/..", fmt.Errorf("get project: %w",
		&api.InvalidPathSegmentError{Kind: "project identifier", Value: "..", Reason: "must not be a dot segment"}))
	if err == nil || !strings.Contains(err.Error(), `Invalid argument: project identifier ".." must not be a dot segment`) {
		t.Fatalf("resourceError() = %v", err)
	}
}
