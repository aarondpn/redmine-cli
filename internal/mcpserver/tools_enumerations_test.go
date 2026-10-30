package mcpserver

import (
	"context"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestListPrioritiesTool_RoundTrip(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/enumerations/issue_priorities.json" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issue_priorities":[{"id":2,"name":"Normal","is_default":true,"active":true}]}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_priorities"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	text := asText(t, res.Content[0])
	if !containsText(text, `"issue_priorities"`, `"name":"Normal"`, `"count":1`) {
		t.Fatalf("unexpected tool payload: %s", text)
	}
}

func TestListTimeEntryActivitiesTool_RoundTrip(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/enumerations/time_entry_activities.json":
			_, _ = w.Write([]byte(`{"time_entry_activities":[{"id":9,"name":"Design","active":true}]}`))
		case "/projects/demo.json":
			if r.URL.Query().Get("include") != "time_entry_activities" {
				http.Error(w, "missing include", http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"demo","name":"Demo","time_entry_activities":[{"id":50,"name":"Design"}]}}`))
		default:
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
		}
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_time_entry_activities"})
	if err != nil || res.IsError {
		t.Fatalf("global call: err=%v res=%+v", err, res)
	}
	if text := asText(t, res.Content[0]); !containsText(text, `"id":9`, `"time_entry_activities"`) {
		t.Fatalf("global payload: %s", text)
	}

	res, err = cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_time_entry_activities",
		Arguments: map[string]any{"project_id": "demo"},
	})
	if err != nil || res.IsError {
		t.Fatalf("project call: err=%v res=%+v", err, res)
	}
	if text := asText(t, res.Content[0]); !containsText(text, `"id":50`, `"count":1`) {
		t.Fatalf("project payload: %s", text)
	}
}

func TestEnumerationTools_Groups(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer closeTS()

	cases := []struct {
		group   Group
		present string
		absent  string
	}{
		{GroupMeta, "list_priorities", "list_time_entry_activities"},
		{GroupTime, "list_time_entry_activities", "list_priorities"},
	}
	for _, tc := range cases {
		opts := ResolveOptions(Options{Version: "v0"}, FilterSpec{EnableGroups: []Group{tc.group}})
		cs, cleanup := newConnectedSession(t, apiClient, opts)
		names := listToolNames(t, cs)
		cleanup()
		if !contains(names, tc.present) {
			t.Errorf("group %s: %s missing", tc.group, tc.present)
		}
		if contains(names, tc.absent) {
			t.Errorf("group %s: %s must not be registered", tc.group, tc.absent)
		}
	}
}
