package mcpserver

import (
	"context"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestSearchTool_ReturnsOffsetAndLimit(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"id":1,"title":"t","type":"issue","url":"u","description":"d","datetime":"x"}],"total_count":9,"offset":0,"limit":25}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "search",
		Arguments: map[string]any{"query": "q"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	text := asText(t, res.Content[0])
	if !containsText(text, `"total_count":9`, `"offset":0`, `"limit":25`) {
		t.Errorf("missing page meta: %s", text)
	}
}

func TestListUsersTool_PassesIncludesAndKeepsNewFields(t *testing.T) {
	var gotInclude string
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInclude = r.URL.Query().Get("include")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"users":[{"id":1,"login":"a","status":1,"passwd_changed_on":"2026-01-01T00:00:00Z","twofa_scheme":"totp","auth_source":{"id":2,"name":"LDAP"}}],"total_count":1}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_users",
		Arguments: map[string]any{"includes": []string{"auth_source"}},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	if gotInclude != "auth_source" {
		t.Errorf("include = %q, want auth_source", gotInclude)
	}
	text := asText(t, res.Content[0])
	if !containsText(text, `"auth_source":{`, `"LDAP"`, `"passwd_changed_on"`, `"twofa_scheme":"totp"`) {
		t.Errorf("missing new user fields: %s", text)
	}
}

func TestListMembershipsTool_KeepsInheritedRoles(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"memberships":[{"id":1,"project":{"id":1,"name":"Demo"},"group_anonymous":{"id":13,"name":"Anonymous users"},"roles":[{"id":4,"name":"Developer","inherited":true}]}],"total_count":1}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_memberships",
		Arguments: map[string]any{"project_id": "1"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	text := asText(t, res.Content[0])
	if !containsText(text, `"inherited":true`, `"group_anonymous":{`) {
		t.Errorf("missing membership fields: %s", text)
	}
}
