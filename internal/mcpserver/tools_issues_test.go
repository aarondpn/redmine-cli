package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// captureIssueUpdateHandler records every PUT /issues/{id}.json body in the
// order they arrive and responds 204.
type captureIssueUpdateHandler struct {
	mu     sync.Mutex
	bodies []map[string]any
}

func (h *captureIssueUpdateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "unexpected method "+r.Method, http.StatusMethodNotAllowed)
		return
	}
	body, _ := io.ReadAll(r.Body)
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		http.Error(w, "bad body", http.StatusBadRequest)
		return
	}
	h.mu.Lock()
	h.bodies = append(h.bodies, parsed)
	h.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (h *captureIssueUpdateHandler) lastIssue(t *testing.T) map[string]any {
	t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.bodies) == 0 {
		t.Fatal("no PUT bodies captured")
	}
	issue, ok := h.bodies[len(h.bodies)-1]["issue"].(map[string]any)
	if !ok {
		t.Fatalf("body missing issue key: %+v", h.bodies[len(h.bodies)-1])
	}
	return issue
}

// TestAddIssueComment_DoesNotTouchIssuePrivacy pins the bug fix: the tool must
// not send the issue-level is_private flag, which would silently flip the
// issue's privacy on every comment.
func TestAddIssueComment_DoesNotTouchIssuePrivacy(t *testing.T) {
	cap := &captureIssueUpdateHandler{}
	apiClient, closeTS := newTestAPIClient(t, cap)
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "add_issue_comment",
		Arguments: map[string]any{"id": 42, "notes": "hello"},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected tool error: %+v", res.Content)
	}
	issue := cap.lastIssue(t)
	if _, ok := issue["is_private"]; ok {
		t.Errorf("default comment unexpectedly sent is_private: %+v", issue)
	}
	if _, ok := issue["private_notes"]; ok {
		t.Errorf("default comment unexpectedly sent private_notes: %+v", issue)
	}
	if got, _ := issue["notes"].(string); got != "hello" {
		t.Errorf("notes = %v, want hello", issue["notes"])
	}
}

// TestAddIssueComment_SetsPrivateNotesWhenRequested verifies the fix wires
// private_notes (journal-level) rather than is_private (issue-level).
func TestAddIssueComment_SetsPrivateNotesWhenRequested(t *testing.T) {
	cap := &captureIssueUpdateHandler{}
	apiClient, closeTS := newTestAPIClient(t, cap)
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "add_issue_comment",
		Arguments: map[string]any{"id": 42, "notes": "shh", "private_notes": true},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	issue := cap.lastIssue(t)
	if got, _ := issue["private_notes"].(bool); !got {
		t.Errorf("private_notes = %v, want true", issue["private_notes"])
	}
	if _, ok := issue["is_private"]; ok {
		t.Errorf("unexpectedly sent is_private along with private_notes: %+v", issue)
	}
}

// TestUpdateIssue_RoutesParentIssueID pins that update_issue can now reparent
// (the jsonschema field was previously missing).
func TestUpdateIssue_RoutesParentIssueID(t *testing.T) {
	cap := &captureIssueUpdateHandler{}
	apiClient, closeTS := newTestAPIClient(t, cap)
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "update_issue",
		Arguments: map[string]any{"id": 7, "parent_issue_id": 99},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	issue := cap.lastIssue(t)
	got, ok := issue["parent_issue_id"].(float64)
	if !ok {
		t.Fatalf("parent_issue_id missing or wrong type: %+v", issue)
	}
	if int(got) != 99 {
		t.Errorf("parent_issue_id = %v, want 99", got)
	}
}

func TestUpdateIssue_RoutesAssignedToID(t *testing.T) {
	cap := &captureIssueUpdateHandler{}
	apiClient, closeTS := newTestAPIClient(t, cap)
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	_, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "update_issue",
		Arguments: map[string]any{"id": 7, "assigned_to_id": 123},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	issue := cap.lastIssue(t)
	got, ok := issue["assigned_to_id"].(float64)
	if !ok {
		t.Fatalf("assigned_to_id missing or wrong type: %+v", issue)
	}
	if int(got) != 123 {
		t.Errorf("assigned_to_id = %v, want 123", got)
	}
}

// TestListIssues_EmptyResultIsArray pins that an empty list encodes as
// "issues":[] rather than null, matching `redmine issues list -o json`.
func TestListIssues_EmptyResultIsArray(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[],"total_count":0,"offset":0,"limit":50}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_issues"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	text := asText(t, res.Content[0])
	if !containsText(text, `"issues":[]`) {
		t.Errorf("empty list must encode as []: %s", text)
	}
	structured, err := json.Marshal(res.StructuredContent)
	if err != nil {
		t.Fatalf("marshal structured content: %v", err)
	}
	if !containsText(string(structured), `"issues":[]`) {
		t.Errorf("structured content must encode empty list as []: %s", structured)
	}
}

// TestGetIssue_EmptyIncludedCollectionsAreArrays pins the JSON contract shared
// with `redmine issues get -o json`: collections that were requested but are
// empty encode as [], while unrequested ones stay absent.
func TestGetIssue_EmptyIncludedCollectionsAreArrays(t *testing.T) {
	var gotInclude string
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInclude = r.URL.Query().Get("include")
		w.Header().Set("Content-Type", "application/json")
		// Mirrors Redmine: journals is sent as [] while children (leaf issue)
		// and relations (none) are dropped even though they were requested.
		_, _ = w.Write([]byte(`{"issue":{"id":7,"subject":"Leaf","journals":[]}}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_issue",
		Arguments: map[string]any{"id": 7, "includes": []string{"journals", "children", "relations"}},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	if gotInclude != "journals,children,relations" {
		t.Errorf("include = %q, want journals,children,relations", gotInclude)
	}
	text := asText(t, res.Content[0])
	if !containsText(text, `"journals":[]`, `"children":[]`, `"relations":[]`, `"custom_fields":[]`) {
		t.Errorf("requested empty collections must encode as []: %s", text)
	}
	for _, key := range []string{`"attachments"`, `"watchers"`} {
		if containsText(text, key) {
			t.Errorf("unrequested collection %s must be absent: %s", key, text)
		}
	}
}
