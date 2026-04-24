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

func TestUpdateVersionTool_RoundTrip(t *testing.T) {
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
		w.WriteHeader(http.StatusNoContent)
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	newStatus := "closed"
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name: "update_version",
		Arguments: map[string]any{
			"id":     9,
			"status": newStatus,
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	if capturedMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", capturedMethod)
	}
	if capturedPath != "/versions/9.json" {
		t.Errorf("path = %s, want /versions/9.json", capturedPath)
	}
	version, ok := capturedBody["version"].(map[string]any)
	if !ok {
		t.Fatalf("body missing version key: %+v", capturedBody)
	}
	if got, _ := version["status"].(string); got != newStatus {
		t.Errorf("status = %q, want %q", got, newStatus)
	}
	// Pointer-nil fields must not appear in the PUT body.
	for _, field := range []string{"name", "sharing", "due_date", "description", "wiki_page_title"} {
		if _, ok := version[field]; ok {
			t.Errorf("omitted field %q unexpectedly sent: %+v", field, version)
		}
	}
}

func TestDeleteVersionTool_RoundTrip(t *testing.T) {
	var capturedMethod, capturedPath string

	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer closeTS()

	cs, cleanup := newConnectedSession(t, apiClient, Options{EnableWrites: true, Version: "v0"})
	defer cleanup()

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "delete_version",
		Arguments: map[string]any{"id": 11},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("tool returned error: %+v", res.Content)
	}
	if capturedMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", capturedMethod)
	}
	if capturedPath != "/versions/11.json" {
		t.Errorf("path = %s, want /versions/11.json", capturedPath)
	}
}
