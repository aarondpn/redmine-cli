package testutil

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// RouteServer starts a test server that answers "METHOD /path" keys from
// routes with their JSON reply (an empty reply is a 204, a POST reply is a
// 201) and fails the test on any other request. The decoded body of every
// POST and PUT is appended to *writes.
func RouteServer(t *testing.T, routes map[string]string, writes *[]map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reply, ok := routes[r.Method+" "+r.URL.Path]
		if !ok {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.Method == http.MethodPost || r.Method == http.MethodPut {
			raw, _ := io.ReadAll(r.Body)
			var body map[string]any
			_ = json.Unmarshal(raw, &body)
			*writes = append(*writes, body)
		}
		if reply == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodPost {
			w.WriteHeader(http.StatusCreated)
		}
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	return srv
}

// RequireOneWrite asserts that writes holds exactly one body equal to want
// (JSON numbers decode as float64).
func RequireOneWrite(t *testing.T, writes []map[string]any, want map[string]any) {
	t.Helper()
	if len(writes) != 1 {
		t.Fatalf("writes = %v, want exactly one", writes)
	}
	if !reflect.DeepEqual(writes[0], want) {
		t.Errorf("body = %v\nwant   %v", writes[0], want)
	}
}
