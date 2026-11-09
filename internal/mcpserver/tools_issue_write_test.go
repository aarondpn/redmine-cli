package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// issueWriteHandler records request bodies by "METHOD path" and answers
// issue GETs with an issue in project getProject.
type issueWriteHandler struct {
	mu         sync.Mutex
	putStatus  int
	putBody    string
	getProject int
	bodies     map[string]map[string]any
}

func (h *issueWriteHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	key := r.Method + " " + r.URL.Path
	h.mu.Lock()
	if h.bodies == nil {
		h.bodies = map[string]map[string]any{}
	}
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	h.bodies[key] = body
	h.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodPut:
		status := h.putStatus
		if status == 0 {
			status = http.StatusNoContent
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(h.putBody))
	case http.MethodGet:
		_ = json.NewEncoder(w).Encode(map[string]any{"issue": map[string]any{"id": 5, "project": map[string]any{"id": h.getProject, "name": "P"}}})
	case http.MethodPost:
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"issue":{"id":9,"subject":"Copy","project":{"id":4,"name":"Other"}}}`))
	}
}

func callIssueTool(t *testing.T, h *issueWriteHandler, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	apiClient, closeTS := newTestAPIClient(t, h)
	t.Cleanup(closeTS)
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	t.Cleanup(cleanup)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	return res
}

func TestUpdateIssue_MoveAndDeleteAttachments(t *testing.T) {
	h := &issueWriteHandler{getProject: 4}
	res := callIssueTool(t, h, "update_issue", map[string]any{
		"id": 5, "project_id": 4, "deleted_attachment_ids": []int{8, 9}, "done_ratio": 50,
	})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	issue := h.bodies["PUT /issues/5.json"]["issue"].(map[string]any)
	got, _ := json.Marshal(issue)
	if string(got) != `{"deleted_attachment_ids":[8,9],"done_ratio":50,"project_id":4}` {
		t.Fatalf("PUT issue = %s", got)
	}
}

func TestUpdateIssue_MoveIgnoredIsToolError(t *testing.T) {
	h := &issueWriteHandler{getProject: 1}
	res := callIssueTool(t, h, "update_issue", map[string]any{"id": 5, "project_id": 4})
	if !res.IsError {
		t.Fatal("expected a tool error when Redmine ignores the move")
	}
	if text := asText(t, res.Content[0]); !strings.Contains(text, "was not moved to project ID 4") {
		t.Fatalf("error = %s", text)
	}
}

func TestCreateIssue_DoneRatio(t *testing.T) {
	h := &issueWriteHandler{}
	res := callIssueTool(t, h, "create_issue", map[string]any{"project_id": 1, "subject": "s", "done_ratio": 30})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	if got := h.bodies["POST /issues.json"]["issue"].(map[string]any)["done_ratio"]; got != float64(30) {
		t.Fatalf("done_ratio = %v", got)
	}
}

func TestCopyIssue_Tool(t *testing.T) {
	h := &issueWriteHandler{getProject: 6}
	res := callIssueTool(t, h, "copy_issue", map[string]any{"id": 5, "copy_attachments": true, "subject": "Copy", "assigned_to_id": 0})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	body := h.bodies["POST /issues.json"]
	if body["copy_from"] != float64(5) || body["copy_attachments"] != "1" || body["project_id"] != float64(6) {
		t.Fatalf("POST body = %v", body)
	}
	if _, ok := body["link_copy"]; ok {
		t.Fatalf("link_copy sent without being requested: %v", body)
	}
	// assigned_to_id 0 must reach Redmine as "" to leave the copy unassigned.
	if got, ok := body["issue"].(map[string]any)["assigned_to_id"]; !ok || got != "" {
		t.Fatalf("assigned_to_id = %#v (present %v), want \"\"", got, ok)
	}
	if text := asText(t, res.Content[0]); !strings.Contains(text, `"id":9`) {
		t.Fatalf("result = %s", text)
	}
}

// Uploads stay out of the MCP issue tools on purpose.
func TestIssueWriteTools_NoUploads(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, &issueWriteHandler{})
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	tools, err := cs.ListTools(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, tool := range tools.Tools {
		switch tool.Name {
		case "create_issue", "update_issue", "copy_issue":
			seen++
			schema, _ := json.Marshal(tool.InputSchema)
			if strings.Contains(string(schema), "upload") {
				t.Errorf("%s exposes uploads: %s", tool.Name, schema)
			}
		}
	}
	if seen != 3 {
		t.Fatalf("found %d of the 3 issue write tools", seen)
	}
}

func TestCopyIssue_LinkCopy(t *testing.T) {
	h := &issueWriteHandler{getProject: 6}
	res := callIssueTool(t, h, "copy_issue", map[string]any{"id": 5, "link_copy": true})
	if res.IsError {
		t.Fatalf("tool error: %+v", res.Content)
	}
	if got := h.bodies["POST /issues.json"]["link_copy"]; got != "1" {
		t.Fatalf("link_copy = %#v, want \"1\"", got)
	}
}
