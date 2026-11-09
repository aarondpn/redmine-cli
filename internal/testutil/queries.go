package testutil

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"
)

// QueryRecorder is a test server that answers every request with a fixed
// JSON body and records each request's decoded query string.
type QueryRecorder struct {
	URL string

	mu      sync.Mutex
	queries []url.Values
}

// NewQueryRecorder starts a QueryRecorder replying with reply. The server is
// closed when the test ends.
func NewQueryRecorder(t *testing.T, reply string) *QueryRecorder {
	t.Helper()
	rec := &QueryRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.mu.Lock()
		rec.queries = append(rec.queries, r.URL.Query())
		rec.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reply))
	}))
	t.Cleanup(srv.Close)
	rec.URL = srv.URL
	return rec
}

// Queries returns the recorded queries in request order.
func (r *QueryRecorder) Queries() []url.Values {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.queries)
}

// Last returns the query of the latest request and fails the test when none
// was made.
func (r *QueryRecorder) Last(t *testing.T) url.Values {
	t.Helper()
	qs := r.Queries()
	if len(qs) == 0 {
		t.Fatal("no request recorded")
	}
	return qs[len(qs)-1]
}

// WantQueryValues fails the test unless q carries exactly want for key, in
// order.
func WantQueryValues(t *testing.T, q url.Values, key string, want ...string) {
	t.Helper()
	if !slices.Equal(q[key], want) {
		t.Errorf("%s = %q, want %q (query %s)", key, q[key], want, q.Encode())
	}
}
