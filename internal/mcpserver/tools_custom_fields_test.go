package mcpserver

import (
	"context"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestListCustomFieldsTool_RoundTrip(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom_fields.json" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"custom_fields":[{"id":1,"name":"Severity","customized_type":"issue","field_format":"list","is_required":true}]}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "list_custom_fields"})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	text := asText(t, res.Content[0])
	if !containsText(text, `"name":"Severity"`, `"customized_type":"issue"`, `"field_format":"list"`) {
		t.Fatalf("unexpected tool payload: %s", text)
	}
}

func TestGetCustomFieldTool_RoundTrip(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom_fields.json" {
			http.Error(w, "unexpected path "+r.URL.Path, http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"custom_fields":[
			{"id":1,"name":"Severity","customized_type":"issue","field_format":"list","possible_values":[{"value":"High","label":"High"}]},
			{"id":2,"name":"Department","customized_type":"user","field_format":"string"}
		]}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_custom_field",
		Arguments: map[string]any{"id": 2},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	text := asText(t, res.Content[0])
	if !containsText(text, `"id":2`, `"name":"Department"`, `"field_format":"string"`) {
		t.Fatalf("unexpected tool payload: %s", text)
	}
}
