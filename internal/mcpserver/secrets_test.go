package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// sentinelAPIKey is what the stub Redmine returns as api_key. It must never
// appear in any MCP output.
const sentinelAPIKey = "SUPER_SECRET_API_KEY_SENTINEL"

// apiKeyLeakingRedmine mimics a Redmine that returns api_key on every user
// endpoint, which /my/account.json always does and the /users endpoints do
// for admin callers on some versions.
func apiKeyLeakingRedmine(t *testing.T) http.Handler {
	t.Helper()
	user := `{"id":1,"login":"alice","firstname":"Alice","lastname":"Doe","mail":"alice@example.com","api_key":"` + sentinelAPIKey + `","status":1}`
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/my/account.json",
			r.Method == http.MethodGet && r.URL.Path == "/users/current.json",
			r.Method == http.MethodGet && r.URL.Path == "/users/1.json":
			_, _ = w.Write([]byte(`{"user":` + user + `}`))
		case r.Method == http.MethodGet && r.URL.Path == "/users.json":
			_, _ = w.Write([]byte(`{"users":[` + user + `],"total_count":1,"offset":0,"limit":50}`))
		case r.Method == http.MethodPost && r.URL.Path == "/users.json":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"user":` + user + `}`))
		default:
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	})
}

// TestMCPNeverReturnsAPIKey is the regression test for H-01: no tool call or
// resource read may hand the Redmine API key to an MCP client, whether in the
// text content or in the structured result.
func TestMCPNeverReturnsAPIKey(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, apiKeyLeakingRedmine(t))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0", EnableWrites: true})
	defer cleanup()
	ctx := context.Background()

	tools := []struct {
		name string
		args map[string]any
	}{
		{"get_my_account", map[string]any{}},
		{"me", map[string]any{}},
		{"get_user", map[string]any{"id": 1}},
		{"list_users", map[string]any{}},
		{"create_user", map[string]any{"login": "alice", "firstname": "Alice", "lastname": "Doe", "mail": "alice@example.com"}},
	}
	for _, tc := range tools {
		t.Run("tool/"+tc.name, func(t *testing.T) {
			res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: tc.name, Arguments: tc.args})
			if err != nil {
				t.Fatalf("CallTool: %v", err)
			}
			if res.IsError {
				t.Fatalf("tool returned error: %+v", res.Content)
			}
			raw, err := json.Marshal(res)
			if err != nil {
				t.Fatalf("marshal result: %v", err)
			}
			if !strings.Contains(string(raw), "alice") {
				t.Fatalf("result does not look like the stub user: %s", raw)
			}
			if strings.Contains(string(raw), sentinelAPIKey) || strings.Contains(string(raw), `"api_key"`) {
				t.Fatalf("tool %s leaked the API key: %s", tc.name, raw)
			}
		})
	}

	for _, uri := range []string{"redmine://user/me", "redmine://user/1"} {
		t.Run("resource/"+uri, func(t *testing.T) {
			res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
			if err != nil {
				t.Fatalf("ReadResource: %v", err)
			}
			raw, err := json.Marshal(res)
			if err != nil {
				t.Fatalf("marshal result: %v", err)
			}
			if !strings.Contains(string(raw), "alice") {
				t.Fatalf("result does not look like the stub user: %s", raw)
			}
			if strings.Contains(string(raw), sentinelAPIKey) || strings.Contains(string(raw), "api_key") {
				t.Fatalf("resource %s leaked the API key: %s", uri, raw)
			}
		})
	}
}

// TestGetMyAccountDescriptionDoesNotAdvertiseAPIKey keeps the tool catalog
// from inviting a model to ask for the key.
func TestGetMyAccountDescriptionDoesNotAdvertiseAPIKey(t *testing.T) {
	for _, d := range AllTools() {
		if strings.Contains(d.Description, "api_key") {
			t.Errorf("tool %q description mentions api_key: %q", d.Name, d.Description)
		}
	}
}

// TestToolOutputSchemasDoNotAdvertiseAPIKey keeps the output schemas in line
// with the redacted results: no schema may declare a field toolResult always
// clears.
func TestToolOutputSchemasDoNotAdvertiseAPIKey(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, apiKeyLeakingRedmine(t))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0", EnableWrites: true})
	defer cleanup()

	res, err := cs.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	userTools := map[string]bool{"get_my_account": false, "me": false, "get_user": false, "list_users": false, "create_user": false}
	for _, tool := range res.Tools {
		raw, err := json.Marshal(tool.OutputSchema)
		if err != nil {
			t.Fatalf("marshal %s output schema: %v", tool.Name, err)
		}
		if strings.Contains(string(raw), `"api_key"`) {
			t.Errorf("tool %s output schema declares api_key: %s", tool.Name, raw)
		}
		if _, ok := userTools[tool.Name]; ok {
			userTools[tool.Name] = strings.Contains(string(raw), `"login"`)
		}
	}
	for name, hasLogin := range userTools {
		if !hasLogin {
			t.Errorf("tool %s output schema lost the other user fields", name)
		}
	}
}

// TestTrimmedOutputSchemaToolErrorsStayToolErrors guards the error path of
// tools whose output schema is set explicitly: a failed call returns a typed
// nil *models.User, which must reach the client as an error result, not as an
// output validation failure.
func TestTrimmedOutputSchemaToolErrorsStayToolErrors(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.NotFoundHandler())
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_user", Arguments: map[string]any{"id": 1}})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Fatalf("get_user against a 404 succeeded: %+v", res.Content)
	}
}
