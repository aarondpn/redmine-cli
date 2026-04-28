package mcpserver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBuildHTTPHandler_InitializeAndListPrompts(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer closeTS()

	handler := BuildHTTPHandler(apiClient, Options{Version: "v0"})
	server := httptest.NewServer(handler)
	defer server.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "v0"}, nil)
	transport := &mcp.StreamableClientTransport{
		Endpoint:             server.URL,
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}
	session, err := client.Connect(context.Background(), transport, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer session.Close()

	prompts, err := session.ListPrompts(context.Background(), nil)
	if err != nil {
		t.Fatalf("ListPrompts over HTTP: %v", err)
	}
	if len(prompts.Prompts) == 0 {
		t.Fatal("expected prompts over HTTP")
	}
}
