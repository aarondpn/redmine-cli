package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

func TestReadOnlyBlocksWritesAllowsReads(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	cfg := &config.Config{Server: srv.URL, APIKey: "k", AuthMethod: "apikey", ReadOnly: true}
	c, err := NewClient(cfg, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}

	// Write is refused before reaching the server.
	err = c.Post(context.Background(), "/issues.json", map[string]any{}, nil)
	var roErr *ErrReadOnly
	if !errors.As(err, &roErr) {
		t.Fatalf("Post error = %v, want *ErrReadOnly", err)
	}
	if hits != 0 {
		t.Fatalf("server received %d requests, want 0", hits)
	}

	// Read still works.
	if err := c.Get(context.Background(), "/issues.json", nil, &struct{}{}); err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if hits != 1 {
		t.Fatalf("server received %d requests, want 1", hits)
	}
}
