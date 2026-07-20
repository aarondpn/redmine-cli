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

	// Writes are refused before reaching the server.
	ctx := context.Background()
	writes := map[string]error{
		"Post":   c.Post(ctx, "/issues.json", map[string]any{}, nil),
		"Put":    c.Put(ctx, "/issues/1.json", map[string]any{}),
		"Delete": c.Delete(ctx, "/issues/1.json"),
	}
	_, rawErr := c.DoRaw(ctx, http.MethodPatch, "/issues/1.json", nil, nil)
	writes["DoRaw PATCH"] = rawErr
	for name, err := range writes {
		var roErr *ErrReadOnly
		if !errors.As(err, &roErr) {
			t.Errorf("%s error = %v, want *ErrReadOnly", name, err)
		}
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
