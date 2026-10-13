package mcpserver

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// silentStatusHandler mimics Redmine dropping a disallowed status transition:
// the PUT succeeds with 204, but the issue keeps its old status.
func silentStatusHandler(t *testing.T) http.Handler {
	t.Helper()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/issues/7.json":
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodGet && r.URL.Path == "/issues/7.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issue":{"id":7,"status":{"id":1,"name":"New"},"allowed_statuses":[{"id":1,"name":"New","is_closed":false},{"id":2,"name":"In Progress","is_closed":false}]}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/issue_statuses.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"issue_statuses":[{"id":1,"name":"New","is_closed":false},{"id":2,"name":"In Progress","is_closed":false},{"id":5,"name":"Closed","is_closed":true}]}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
}

func TestStatusChangeNotApplied_IsToolError(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, silentStatusHandler(t))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	for _, tc := range []struct {
		tool string
		args map[string]any
	}{
		{"update_issue", map[string]any{"id": 7, "status_id": 5, "notes": "closing"}},
		{"close_issue", map[string]any{"id": 7}},
	} {
		t.Run(tc.tool, func(t *testing.T) {
			res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if !res.IsError {
				t.Fatalf("expected a tool error when the status was not applied, got %+v", res)
			}
			text := res.Content[0].(*mcp.TextContent).Text
			for _, want := range []string{`not changed to "Closed"`, "workflow", "Allowed statuses: New (1), In Progress (2)."} {
				if !strings.Contains(text, want) {
					t.Errorf("tool error missing %q:\n%s", want, text)
				}
			}
		})
	}
}
