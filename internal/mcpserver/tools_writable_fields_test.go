package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestWritableFields_InputSchemas pins the new fields of the write tools and
// that update_user has no way to carry a password.
func TestWritableFields_InputSchemas(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.NotFoundHandler())
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		t.Fatal(err)
	}
	schemas := map[string]map[string]any{}
	for _, tool := range res.Tools {
		raw, _ := json.Marshal(tool.InputSchema)
		var s map[string]any
		_ = json.Unmarshal(raw, &s)
		schemas[tool.Name] = s
	}
	props := func(tool string) map[string]any {
		p, _ := schemas[tool]["properties"].(map[string]any)
		return p
	}

	want := map[string][]string{
		"create_user":       {"status", "language", "pref"},
		"update_user":       {"language", "group_ids", "notified_project_ids", "pref"},
		"update_my_account": {"language", "notified_project_ids", "pref"},
		"create_group":      {"twofa_required"},
		"update_group":      {"twofa_required"},
		"create_project":    {"default_issue_query_id"},
		"update_project":    {"default_issue_query_id"},
		"create_version":    {"default_project_version"},
		"update_version":    {"default_project_version"},
		"create_membership": {"user_id", "user_ids"},
	}
	for tool, fields := range want {
		for _, field := range fields {
			if props(tool)[field] == nil {
				t.Errorf("%s input schema lacks %q", tool, field)
			}
		}
	}

	for _, field := range []string{"password", "password_confirmation", "Password"} {
		if props("update_user")[field] != nil {
			t.Errorf("update_user must not offer %q", field)
		}
	}
	pref, _ := props("update_user")["pref"].(map[string]any)
	if pref["type"] != "object" {
		t.Errorf("pref schema = %v, want an object", pref)
	}
	if ap, _ := pref["additionalProperties"].(map[string]any); ap["type"] != "string" {
		t.Errorf("pref values = %v, want strings", pref["additionalProperties"])
	}
	for _, field := range schemas["create_membership"]["required"].([]any) {
		if field == "user_id" {
			t.Error("create_membership: user_id must be optional now that user_ids exists")
		}
	}
}

func TestUpdateUserTool_RejectsPasswordAndSendsPref(t *testing.T) {
	var mu sync.Mutex
	var bodies []map[string]any
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		mu.Lock()
		bodies = append(bodies, body)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	}))
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "update_user", Arguments: map[string]any{"id": 7, "password": "s3cret"}})
	if err == nil && !res.IsError {
		t.Fatalf("update_user accepted a password: %+v", res.Content)
	}
	if len(bodies) != 0 {
		t.Fatalf("a password argument must not reach Redmine: %v", bodies)
	}

	res, err = cs.CallTool(ctx, &mcp.CallToolParams{Name: "update_user", Arguments: map[string]any{
		"id": 7, "language": "de", "group_ids": []int{}, "notified_project_ids": []int{3},
		"pref": map[string]any{"hide_mail": "1", "time_zone": "Berlin"},
	}})
	if err != nil || res.IsError {
		t.Fatalf("update_user: %v %+v", err, res)
	}
	want := map[string]any{
		"user": map[string]any{"language": "de", "group_ids": []any{}, "notified_project_ids": []any{3.0}},
		"pref": map[string]any{"hide_mail": "1", "time_zone": "Berlin"},
	}
	if len(bodies) != 1 || !reflect.DeepEqual(bodies[0], want) {
		t.Errorf("bodies = %v\nwant %v", bodies, want)
	}
}

func TestCreateMembershipTool_UserIDs(t *testing.T) {
	var posted map[string]any
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &posted)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"membership":{"id":11,"project":{"id":1,"name":"Demo"},"user":{"id":5,"name":"Ann"},"roles":[]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"memberships":[{"id":11,"project":{"id":1,"name":"Demo"},"user":{"id":5,"name":"Ann"},"roles":[]},` +
			`{"id":12,"project":{"id":1,"name":"Demo"},"user":{"id":6,"name":"Bob"},"roles":[]}],"total_count":2}`))
	}))
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "create_membership", Arguments: map[string]any{
		"project_id": "demo", "user_ids": []int{5, 6}, "role_ids": []int{3},
	}})
	if err != nil || res.IsError {
		t.Fatalf("create_membership: %v %+v", err, res)
	}
	want := map[string]any{"membership": map[string]any{"user_ids": []any{5.0, 6.0}, "role_ids": []any{3.0}}}
	if !reflect.DeepEqual(posted, want) {
		t.Errorf("body = %v", posted)
	}
}
