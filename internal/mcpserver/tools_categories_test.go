package mcpserver

import (
	"context"
	"io"
	"net/http"
	"sync"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type mcpReq struct{ method, uri, body string }

func recordingHandler(t *testing.T, responses map[string]string) (http.Handler, func() []mcpReq) {
	t.Helper()
	var (
		mu    sync.Mutex
		calls []mcpReq
	)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls = append(calls, mcpReq{r.Method, r.URL.RequestURI(), string(b)})
		mu.Unlock()
		if resp, ok := responses[r.Method+" "+r.URL.Path]; ok {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(resp))
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return h, func() []mcpReq {
		mu.Lock()
		defer mu.Unlock()
		return append([]mcpReq(nil), calls...)
	}
}

func TestGetIssueCategoryTool_RoundTrip(t *testing.T) {
	h, _ := recordingHandler(t, map[string]string{
		"GET /issue_categories/4.json": `{"issue_category":{"id":4,"name":"UI","project":{"id":1,"name":"Demo"},"assigned_to":{"id":2,"name":"Alice"}}}`,
	})
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_issue_category", Arguments: map[string]any{"id": 4}})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	if text := asText(t, res.Content[0]); !containsText(text, `"id":4`, `"name":"UI"`, `"Alice"`) {
		t.Fatalf("payload: %s", text)
	}
}

func TestCreateIssueCategoryTool_RoundTrip(t *testing.T) {
	h, calls := recordingHandler(t, map[string]string{
		"POST /projects/demo/issue_categories.json": `{"issue_category":{"id":12,"name":"New","project":{"id":1,"name":"Demo"}}}`,
	})
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "create_issue_category",
		Arguments: map[string]any{"project_id": "demo", "name": "New", "assigned_to_id": 2},
	})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	if got := calls()[0].body; got != `{"issue_category":{"name":"New","assigned_to_id":2}}` {
		t.Errorf("body = %s", got)
	}
	if text := asText(t, res.Content[0]); !containsText(text, `"id":12`) {
		t.Fatalf("payload: %s", text)
	}
}

func TestUpdateIssueCategoryTool_ClearsAssignee(t *testing.T) {
	h, calls := recordingHandler(t, nil)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "update_issue_category",
		Arguments: map[string]any{"id": 4, "assigned_to_id": 0},
	})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	c := calls()[0]
	if c.method != "PUT" || c.uri != "/issue_categories/4.json" || c.body != `{"issue_category":{"assigned_to_id":""}}` {
		t.Errorf("call = %+v", c)
	}
}

func TestDeleteIssueCategoryTool_Reassign(t *testing.T) {
	h, calls := recordingHandler(t, map[string]string{
		"GET /issue_categories/4.json": `{"issue_category":{"id":4,"name":"A","project":{"id":1,"name":"Demo"}}}`,
		"GET /issue_categories/9.json": `{"issue_category":{"id":9,"name":"B","project":{"id":1,"name":"Demo"}}}`,
	})
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "delete_issue_category",
		Arguments: map[string]any{"id": 4, "reassign_to_id": 9},
	})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	all := calls()
	last := all[len(all)-1]
	if last.method != "DELETE" || last.uri != "/issue_categories/4.json?reassign_to_id=9" {
		t.Errorf("last call = %+v", last)
	}
}

func TestDeleteIssueCategoryTool_CrossProjectReassignIsError(t *testing.T) {
	h, calls := recordingHandler(t, map[string]string{
		"GET /issue_categories/4.json": `{"issue_category":{"id":4,"name":"A","project":{"id":1,"name":"Demo"}}}`,
		"GET /issue_categories/9.json": `{"issue_category":{"id":9,"name":"B","project":{"id":2,"name":"Other"}}}`,
	})
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "delete_issue_category",
		Arguments: map[string]any{"id": 4, "reassign_to_id": 9},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatalf("expected tool error, got %+v", res.Content)
	}
	for _, c := range calls() {
		if c.method == "DELETE" {
			t.Errorf("DELETE must not be sent")
		}
	}
}

func TestIssueCategoryWriteToolsHiddenWithoutWrites(t *testing.T) {
	h, _ := recordingHandler(t, nil)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()
	names := listToolNames(t, cs)
	for _, n := range []string{"create_issue_category", "update_issue_category", "delete_issue_category"} {
		if contains(names, n) {
			t.Errorf("%s registered without writes", n)
		}
	}
	if !contains(names, "get_issue_category") {
		t.Error("get_issue_category missing")
	}
}
