package mcpserver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// newTestAPIClient spins up an httptest.Server wired to handler and returns a
// fully-initialised *api.Client pointing at it.
func newTestAPIClient(t *testing.T, handler http.Handler) (*api.Client, func()) {
	t.Helper()
	ts := httptest.NewServer(handler)
	cfg := &config.Config{
		Server:     ts.URL,
		APIKey:     "test-key",
		AuthMethod: "apikey",
	}
	client, err := api.NewClient(cfg, debug.New(nil))
	if err != nil {
		ts.Close()
		t.Fatalf("api.NewClient: %v", err)
	}
	return client, ts.Close
}

// newConnectedSession wires an in-memory client<->server session with the
// given Options and returns the connected client session. Server and client
// sessions are closed via the returned cleanup.
func newConnectedSession(t *testing.T, apiClient *api.Client, opts Options) (*mcp.ClientSession, func()) {
	t.Helper()
	ctx := context.Background()

	srv := BuildServer(apiClient, opts)
	ct, st := mcp.NewInMemoryTransports()
	ss, err := srv.Connect(ctx, st, nil)
	if err != nil {
		t.Fatalf("server Connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, ct, nil)
	if err != nil {
		_ = ss.Close()
		t.Fatalf("client Connect: %v", err)
	}
	return cs, func() {
		_ = cs.Close()
		_ = ss.Close()
	}
}

func TestWriteGate_HidesMutatingTools(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer closeTS()

	readonly, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	names := listToolNames(t, readonly)

	wantMissing := []string{
		"create_issue", "update_issue", "delete_issue",
		"add_issue_comment", "assign_issue", "close_issue", "reopen_issue",
		"create_project", "update_project", "delete_project",
		"create_time_entry", "update_time_entry", "delete_time_entry",
		"create_user", "update_user", "delete_user",
		"create_wiki_page", "update_wiki_page", "delete_wiki_page",
		"create_membership", "update_membership", "delete_membership",
	}
	for _, n := range wantMissing {
		if contains(names, n) {
			t.Errorf("tool %q registered without --enable-writes", n)
		}
	}

	wantPresent := []string{
		"list_issues", "get_issue",
		"list_projects", "get_project", "list_project_members",
		"list_time_entries", "get_time_entry", "summary_time_entries",
		"list_users", "get_user", "me",
		"search",
		"list_versions", "get_version",
		"list_trackers", "list_statuses", "list_categories",
		"list_wiki_pages", "get_wiki_page",
		"list_memberships", "get_membership",
	}
	for _, n := range wantPresent {
		if !contains(names, n) {
			t.Errorf("read tool %q missing", n)
		}
	}
}

func TestWriteGate_RegistersMutatingTools(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer closeTS()

	writable, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	names := listToolNames(t, writable)
	wantPresent := []string{
		"create_issue", "update_issue", "delete_issue",
		"create_project", "delete_project",
		"create_time_entry", "update_time_entry",
		"delete_user",
		"create_wiki_page", "delete_wiki_page",
		"create_membership", "delete_membership",
	}
	for _, n := range wantPresent {
		if !contains(names, n) {
			t.Errorf("write tool %q missing with EnableWrites=true", n)
		}
	}
}

func TestListIssuesTool_RoundTrip(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issues.json" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"issues": [
				{"id": 1, "subject": "First", "project": {"id": 1, "name": "P"}, "tracker": {"id": 1, "name": "Bug"}, "status": {"id": 1, "name": "New"}, "priority": {"id": 1, "name": "Normal"}, "author": {"id": 1, "name": "alice"}, "description": "", "done_ratio": 0, "created_on": "", "updated_on": ""},
				{"id": 2, "subject": "Second", "project": {"id": 1, "name": "P"}, "tracker": {"id": 1, "name": "Bug"}, "status": {"id": 1, "name": "New"}, "priority": {"id": 1, "name": "Normal"}, "author": {"id": 1, "name": "alice"}, "description": "", "done_ratio": 0, "created_on": "", "updated_on": ""}
			],
			"total_count": 2,
			"offset": 0,
			"limit": 25
		}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	ctx := context.Background()
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name:      "list_issues",
		Arguments: map[string]any{"project_id": "demo", "limit": 10},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	if len(res.Content) == 0 {
		t.Fatal("expected content")
	}
	text := asText(t, res.Content[0])
	if !strings.Contains(text, `"total_count":2`) || !strings.Contains(text, `"First"`) {
		t.Errorf("unexpected content body: %s", text)
	}
}

func TestIssueResource_RoundTrip(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/issues/42.json" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issue":{"id":42,"subject":"hello","project":{"id":1,"name":"P"},"tracker":{"id":1,"name":"Bug"},"status":{"id":1,"name":"New"},"priority":{"id":1,"name":"Normal"},"author":{"id":1,"name":"alice"},"description":"","done_ratio":0,"created_on":"","updated_on":""}}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	ctx := context.Background()
	res, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "redmine://issue/42"})
	if err != nil {
		t.Fatalf("ReadResource: %v", err)
	}
	if len(res.Contents) != 1 {
		t.Fatalf("expected 1 contents block, got %d", len(res.Contents))
	}
	body := res.Contents[0].Text
	var decoded struct {
		ID      int    `json:"id"`
		Subject string `json:"subject"`
	}
	if err := json.Unmarshal([]byte(body), &decoded); err != nil {
		t.Fatalf("body not JSON: %v\n%s", err, body)
	}
	if decoded.ID != 42 || decoded.Subject != "hello" {
		t.Errorf("unexpected decoded body: %+v", decoded)
	}
}

func TestIssueResource_NotFound(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"errors":["not found"]}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	ctx := context.Background()
	_, err := cs.ReadResource(ctx, &mcp.ReadResourceParams{URI: "redmine://issue/999999"})
	if err == nil {
		t.Fatal("expected error for missing resource")
	}
}

func TestListResourceTemplates(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	ctx := context.Background()
	res, err := cs.ListResourceTemplates(ctx, nil)
	if err != nil {
		t.Fatalf("ListResourceTemplates: %v", err)
	}
	want := map[string]bool{
		tmplIssue: false, tmplProject: false, tmplUser: false,
		tmplTimeEntry: false, tmplWiki: false, tmplVersion: false,
	}
	for _, rt := range res.ResourceTemplates {
		if _, ok := want[rt.URITemplate]; ok {
			want[rt.URITemplate] = true
		}
	}
	for tmpl, ok := range want {
		if !ok {
			t.Errorf("resource template %q missing", tmpl)
		}
	}
}

// --- helpers ---

func listToolNames(t *testing.T, cs *mcp.ClientSession) []string {
	t.Helper()
	res, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := make([]string, 0, len(res.Tools))
	for _, tool := range res.Tools {
		names = append(names, tool.Name)
	}
	sort.Strings(names)
	return names
}

func contains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

func asText(t *testing.T, c mcp.Content) string {
	t.Helper()
	tc, ok := c.(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", c)
	}
	return tc.Text
}
