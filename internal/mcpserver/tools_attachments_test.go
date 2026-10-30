package mcpserver

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestUpdateAttachmentTool_RoundTrip(t *testing.T) {
	h, calls := recordingHandler(t, nil)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "update_attachment",
		Arguments: map[string]any{"id": 7, "filename": "new.png", "description": ""},
	})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	c := calls()[0]
	if c.method != "PUT" || c.uri != "/attachments/7.json" || c.body != `{"attachment":{"filename":"new.png","description":""}}` {
		t.Errorf("call = %+v", c)
	}
}

func TestUpdateAttachmentTool_NothingToUpdateIsError(t *testing.T) {
	h, calls := recordingHandler(t, nil)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "update_attachment", Arguments: map[string]any{"id": 7}})
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsError {
		t.Fatal("expected tool error")
	}
	if len(calls()) != 0 {
		t.Errorf("unexpected requests: %v", calls())
	}
}

func TestDeleteAttachmentTool_RoundTrip(t *testing.T) {
	h, calls := recordingHandler(t, nil)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "delete_attachment", Arguments: map[string]any{"id": 7}})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	c := calls()[0]
	if c.method != "DELETE" || c.uri != "/attachments/7.json" {
		t.Errorf("call = %+v", c)
	}
}

func TestAttachmentWriteToolsHiddenWithoutWrites(t *testing.T) {
	h, _ := recordingHandler(t, nil)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()
	names := listToolNames(t, cs)
	for _, n := range []string{"update_attachment", "delete_attachment"} {
		if contains(names, n) {
			t.Errorf("%s registered without writes", n)
		}
	}
}
