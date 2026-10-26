package mcpserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHTTPServer_RejectsNonLoopbackHostOnLoopbackBind guards the DNS
// rebinding protection the go-sdk streamable handler applies to loopback
// listeners. The unauthenticated loopback mode (L-09) relies on it: a web page
// that rebinds its own hostname to 127.0.0.1 must not be able to drive the
// server from the user's browser. If an SDK upgrade drops or disables the
// check, this test fails.
func TestHTTPServer_RejectsNonLoopbackHostOnLoopbackBind(t *testing.T) {
	apiClient, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer closeTS()

	const initialize = `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-11-25","capabilities":{},"clientInfo":{"name":"t","version":"0"}}}`

	tests := []struct {
		name       string
		token      string
		host       string
		wantStatus int
	}{
		{"no token, rebinding host", "", "evil.example.com", http.StatusForbidden},
		{"no token, rebinding host with port", "", "evil.example.com:8080", http.StatusForbidden},
		{"valid token, rebinding host", "s3cret", "evil.example.com", http.StatusForbidden},
		{"no token, loopback host", "", "", http.StatusOK},
		{"no token, localhost host", "", "localhost", http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := BuildHTTPServer(apiClient, Options{Version: "v0"}, HTTPOptions{Addr: "127.0.0.1:0", AuthToken: tt.token})
			ts := httptest.NewServer(srv.Handler)
			defer ts.Close()

			req, err := http.NewRequest(http.MethodPost, ts.URL, strings.NewReader(initialize))
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", "application/json, text/event-stream")
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			if tt.host != "" {
				req.Host = tt.host
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("POST: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", resp.StatusCode, tt.wantStatus)
			}
		})
	}
}
