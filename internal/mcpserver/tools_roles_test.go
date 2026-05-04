package mcpserver

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestListRolesTool_RoundTrip(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/roles.json" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"roles":[{"id":1,"name":"Manager","assignable":true},{"id":2,"name":"Reporter","assignable":false}]}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_roles"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	text := asText(t, res.Content[0])
	if !containsText(text, `"name":"Manager"`, `"assignable":true`) {
		t.Fatalf("unexpected tool payload: %s", text)
	}
}

func TestGetRoleTool_RoundTrip(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/roles/5.json" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"role":{"id":5,"name":"Reporter","assignable":true,"permissions":["view_issues","add_issue_notes"]}}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_role",
		Arguments: map[string]any{"id": 5},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	text := asText(t, res.Content[0])
	if !containsText(text, `"id":5`, `"Reporter"`, `"permissions":["view_issues","add_issue_notes"]`) {
		t.Fatalf("unexpected tool payload: %s", text)
	}
}

func containsText(text string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(text, part) {
			return false
		}
	}
	return true
}
