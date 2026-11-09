package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestGetWikiPageTool_Version(t *testing.T) {
	h, calls := recordingHandler(t, map[string]string{
		"GET /projects/demo/wiki/Home/3.json": `{"wiki_page":{"title":"Home","text":"old","version":3}}`,
	})
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_wiki_page",
		Arguments: map[string]any{"project_id": "demo", "page": "Home", "version": 3},
	})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	if text := asText(t, res.Content[0]); !containsText(text, `"version":3`, `"text":"old"`) {
		t.Fatalf("payload: %s", text)
	}
	if got := calls(); len(got) != 1 || got[0].uri != "/projects/demo/wiki/Home/3.json" {
		t.Fatalf("calls = %+v", got)
	}
}

func TestDeleteWikiPageTool_Todo(t *testing.T) {
	h, calls := recordingHandler(t, nil)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "delete_wiki_page",
		Arguments: map[string]any{"project_id": "demo", "page": "Parent", "todo": "destroy"},
	})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	got := calls()
	if len(got) != 1 || got[0].method != "DELETE" || got[0].uri != "/projects/demo/wiki/Parent.json?todo=destroy" {
		t.Fatalf("calls = %+v", got)
	}
}

func TestUpdateWikiPageTool_Options(t *testing.T) {
	h, calls := recordingHandler(t, nil)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "update_wiki_page",
		Arguments: map[string]any{
			"project_id": "demo", "page": "Home", "text": "body", "title": "Start",
			"is_start_page": true, "redirect_existing_links": false,
			"deleted_attachment_ids": []int{5, 6},
		},
	})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	var put string
	for _, c := range calls() {
		if c.method == "PUT" {
			put = c.body
		}
	}
	for _, want := range []string{`"is_start_page":true`, `"redirect_existing_links":"0"`, `"deleted_attachment_ids":[5,6]`} {
		if !strings.Contains(put, want) {
			t.Errorf("PUT body %s missing %s", put, want)
		}
	}
}

func TestSearchTool_Attachments(t *testing.T) {
	h, calls := recordingHandler(t, map[string]string{
		"GET /search.json": `{"results":[],"total_count":0}`,
	})
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "search",
		Arguments: map[string]any{"query": "report", "attachments": "only"},
	})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	got := calls()
	if len(got) != 1 || !strings.Contains(got[0].uri, "attachments=only") {
		t.Fatalf("calls = %+v", got)
	}
}
