package mcpserver

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestUpdateJournalTool_RoundTrip(t *testing.T) {
	h, calls := recordingHandler(t, nil)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "update_journal",
		Arguments: map[string]any{"id": 5, "notes": "", "private_notes": false},
	})
	if err != nil || res.IsError {
		t.Fatalf("err=%v res=%+v", err, res)
	}
	c := calls()[0]
	if c.method != "PUT" || c.uri != "/journals/5.json" || c.body != `{"journal":{"notes":"","private_notes":false}}` {
		t.Errorf("call = %+v", c)
	}
}

func TestUpdateJournalTool_NothingToUpdateIsError(t *testing.T) {
	h, calls := recordingHandler(t, nil)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "update_journal", Arguments: map[string]any{"id": 5}})
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

func TestUpdateJournalToolHiddenWithoutWrites(t *testing.T) {
	h, _ := recordingHandler(t, nil)
	apiClient, closeTS := newTestAPIClient(t, h)
	defer closeTS()
	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()
	if contains(listToolNames(t, cs), "update_journal") {
		t.Error("update_journal registered without writes")
	}
}
