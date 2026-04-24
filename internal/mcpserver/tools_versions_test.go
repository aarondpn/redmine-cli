package mcpserver

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestCreateVersionTool_RoundTrip(t *testing.T) {
	var capturedMethod, capturedPath string
	var capturedBody map[string]any

	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &capturedBody); err != nil {
			http.Error(w, "bad body", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"version":{"id":9,"project":{"id":1,"name":"Demo"},"name":"v2.0","status":"open","due_date":"","sharing":"none","description":"","created_on":"","updated_on":""}}`))
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "create_version",
		Arguments: map[string]any{
			"project_id":  "demo",
			"name":        "v2.0",
			"status":      "open",
			"description": "Release",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	if capturedMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", capturedMethod)
	}
	if capturedPath != "/projects/demo/versions.json" {
		t.Errorf("path = %s, want /projects/demo/versions.json", capturedPath)
	}
	version, ok := capturedBody["version"].(map[string]any)
	if !ok {
		t.Fatalf("body missing version key: %+v", capturedBody)
	}
	if got, _ := version["name"].(string); got != "v2.0" {
		t.Errorf("name = %q, want v2.0", got)
	}
	if got, _ := version["description"].(string); got != "Release" {
		t.Errorf("description = %q, want Release", got)
	}
}
