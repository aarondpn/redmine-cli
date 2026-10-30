package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const mcpNewsRow = `{"id":7,"project":{"id":1,"name":"Demo"},"author":{"id":2,"name":"Alice"},"title":"T","summary":"S","description":"D","created_on":"2024-01-01T00:00:00Z"}`

func TestListNewsTool_RoundTrip(t *testing.T) {
	h, calls := recordingHandler(t, map[string]string{
		"GET /projects/demo/news.json": `{"news":[` + mcpNewsRow + `],"total_count":1}`,
	})
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_news",
		Arguments: map[string]any{"project_id": "demo", "limit": 5},
	})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	if got := calls()[0].uri; !strings.HasPrefix(got, "/projects/demo/news.json?limit=5") {
		t.Errorf("uri = %s", got)
	}
}

func TestListNewsTool_GlobalWithoutProject(t *testing.T) {
	h, calls := recordingHandler(t, map[string]string{"GET /news.json": `{"news":[],"total_count":0}`})
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_news", Arguments: map[string]any{}})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	if got := calls()[0].uri; !strings.HasPrefix(got, "/news.json") {
		t.Errorf("uri = %s", got)
	}
}

func TestGetNewsTool_Includes(t *testing.T) {
	h, calls := recordingHandler(t, map[string]string{
		"GET /news/7.json": `{"news":` + mcpNewsRow + `}`,
	})
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_news",
		Arguments: map[string]any{"id": 7, "includes": []string{"comments", "attachments"}},
	})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	if got := calls()[0].uri; got != "/news/7.json?include=comments%2Cattachments" {
		t.Errorf("uri = %s", got)
	}
}

func TestCreateNewsTool_ReadBackID(t *testing.T) {
	h, calls := recordingHandler(t, map[string]string{
		"GET /users/current.json":      `{"user":{"id":2,"login":"alice","firstname":"Alice","lastname":"A"}}`,
		"GET /projects/demo/news.json": `{"news":[` + mcpNewsRow + `],"total_count":1}`,
	})
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "create_news",
		Arguments: map[string]any{"project_id": "demo", "title": "T", "summary": "S", "description": "D"},
	})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	sc, ok := res.StructuredContent.(map[string]any)
	if !ok || sc["id"] != float64(7) {
		t.Errorf("structured = %#v, want id 7", res.StructuredContent)
	}
	post := calls()[0]
	if post.method != "POST" || post.uri != "/projects/demo/news.json" || post.body != `{"news":{"title":"T","summary":"S","description":"D"}}` {
		t.Errorf("post = %+v", post)
	}
}

func TestCreateNewsTool_NoMatchStillSucceeds(t *testing.T) {
	h, _ := recordingHandler(t, map[string]string{
		"GET /users/current.json":      `{"user":{"id":2,"login":"alice","firstname":"Alice","lastname":"A"}}`,
		"GET /projects/demo/news.json": `{"news":[],"total_count":0}`,
	})
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "create_news",
		Arguments: map[string]any{"project_id": "demo", "title": "T", "description": "D"},
	})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	var text string
	for _, c := range res.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			text += tc.Text
		}
	}
	if !strings.Contains(text, "could not be determined") {
		t.Errorf("text = %q, want the read-back warning", text)
	}
}

func TestUpdateAndDeleteNewsTools(t *testing.T) {
	h, calls := recordingHandler(t, nil)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	for _, c := range []mcp.CallToolParams{
		{Name: "update_news", Arguments: map[string]any{"id": 7, "title": "New"}},
		{Name: "delete_news", Arguments: map[string]any{"id": 7}},
	} {
		res, err := cs.CallTool(context.Background(), &c)
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v res=%+v", c.Name, err, res)
		}
	}
	got := calls()
	if len(got) != 2 || got[0].method != "PUT" || got[0].uri != "/news/7.json" || got[0].body != `{"news":{"title":"New"}}` ||
		got[1].method != "DELETE" || got[1].uri != "/news/7.json" {
		t.Errorf("calls = %+v", got)
	}
}

func TestNewsWriteToolsHiddenWithoutWrites(t *testing.T) {
	h, _ := recordingHandler(t, nil)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()
	names := listToolNames(t, cs)
	for _, n := range []string{"create_news", "update_news", "delete_news"} {
		if contains(names, n) {
			t.Errorf("%s registered without writes", n)
		}
	}
	for _, n := range []string{"list_news", "get_news"} {
		if !contains(names, n) {
			t.Errorf("%s missing", n)
		}
	}
}
