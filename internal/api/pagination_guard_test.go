package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
)

// countingHandler wraps h and counts the requests it serves.
func countingHandler(n *atomic.Int32, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n.Add(1)
		h.ServeHTTP(w, r)
	})
}

// lyingTotalHandler serves the first `real` items correctly but always claims
// total_count is `claimed`.
func lyingTotalHandler(key string, real, claimed int) http.Handler {
	items := makeItems(real, "open")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		page := []testItem{}
		if offset < len(items) {
			page = items[offset:min(offset+limit, len(items))]
		}
		_ = json.NewEncoder(w).Encode(map[string]any{key: page, "total_count": claimed})
	})
}

// offsetIgnoringHandler always returns the same full page and claims a huge
// total_count.
func offsetIgnoringHandler(key string) http.Handler {
	items := makeItems(100, "open")
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{key: items, "total_count": 1_000_000})
	})
}

// endlessHandler returns distinct full pages forever. Items carry no "id", so
// only the page cap can stop it.
func endlessHandler(key string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		page := make([]map[string]string, limit)
		for i := range page {
			page[i] = map[string]string{"status": strconv.Itoa(offset + i)}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{key: page, "total_count": 1 << 40})
	})
}

// limitIgnoringHandler honors offset but always returns up to perPage items,
// whatever limit was requested.
func limitIgnoringHandler(key string, total, perPage int) http.Handler {
	items := makeItems(total, "open")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		page := []testItem{}
		if offset < len(items) {
			page = items[offset:min(offset+perPage, len(items))]
		}
		_ = json.NewEncoder(w).Encode(map[string]any{key: page, "total_count": total})
	})
}

// assertDistinctSequence checks that got holds the items with IDs 1..want in
// order, i.e. that no page was counted twice.
func assertDistinctSequence(t *testing.T, got []testItem, want int) {
	t.Helper()
	if len(got) != want {
		t.Fatalf("len(got) = %d, want %d", len(got), want)
	}
	for i, it := range got {
		if it.ID != i+1 {
			t.Fatalf("got[%d].ID = %d, want %d (duplicated or skipped items)", i, it.ID, i+1)
		}
	}
}

// A server that ignores limit returns longer pages than requested; since
// the next request starts pageSize items further on, keeping the whole page
// would repeat most of it on every page.
func TestFetchAll_ServerIgnoresLimit(t *testing.T) {
	var reqs atomic.Int32
	ts := httptest.NewServer(countingHandler(&reqs, limitIgnoringHandler("things", 1000, 250)))
	defer ts.Close()

	got, total, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertDistinctSequence(t, got, 1000)
	if total != 1000 {
		t.Errorf("total = %d, want 1000", total)
	}
	if n := reqs.Load(); n != 10 {
		t.Errorf("requests = %d, want 10", n)
	}
}

func TestFetchAll_ServerIgnoresLimitWithMaxResults(t *testing.T) {
	ts := httptest.NewServer(limitIgnoringHandler("things", 1000, 250))
	defer ts.Close()

	got, _, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 150)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertDistinctSequence(t, got, 150)
}

// The page cap bounds the items kept, not just the requests made, even
// when every page is far longer than requested.
func TestFetchAll_PageCapBoundsItems(t *testing.T) {
	old := maxPages
	maxPages = 3
	t.Cleanup(func() { maxPages = old })

	ts := httptest.NewServer(limitIgnoringHandler("things", 100_000, 5000))
	defer ts.Close()

	got, _, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertDistinctSequence(t, got, 300)
}

func TestFetchAllFiltered_ServerIgnoresLimit(t *testing.T) {
	var reqs atomic.Int32
	ts := httptest.NewServer(countingHandler(&reqs, limitIgnoringHandler("things", 1000, 250)))
	defer ts.Close()

	got, hasMore, err := FetchAllFiltered[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0,
		func(testItem) bool { return true })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertDistinctSequence(t, got, 1000)
	if hasMore {
		t.Error("hasMore = true, want false")
	}
	if n := reqs.Load(); n != 10 {
		t.Errorf("requests = %d, want 10", n)
	}
}

func TestFetchAll_StopsOnEmptyPageDespiteTotalCount(t *testing.T) {
	var reqs atomic.Int32
	ts := httptest.NewServer(countingHandler(&reqs, lyingTotalHandler("things", 200, 5000)))
	defer ts.Close()

	got, _, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 200 {
		t.Errorf("len(got) = %d, want 200", len(got))
	}
	if n := reqs.Load(); n != 3 {
		t.Errorf("requests = %d, want 3 (two full pages, then an empty one)", n)
	}
}

func TestFetchAll_StopsWhenServerIgnoresOffset(t *testing.T) {
	var reqs atomic.Int32
	ts := httptest.NewServer(countingHandler(&reqs, offsetIgnoringHandler("things")))
	defer ts.Close()

	got, _, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 100 {
		t.Errorf("len(got) = %d, want 100 (the repeated page must not be appended)", len(got))
	}
	if n := reqs.Load(); n != 2 {
		t.Errorf("requests = %d, want 2", n)
	}
}

func TestFetchAll_PageCap(t *testing.T) {
	old := maxPages
	maxPages = 3
	t.Cleanup(func() { maxPages = old })

	var reqs atomic.Int32
	ts := httptest.NewServer(countingHandler(&reqs, endlessHandler("things")))
	defer ts.Close()

	got, _, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n := reqs.Load(); n != 3 {
		t.Errorf("requests = %d, want 3 (page cap)", n)
	}
	if len(got) != 300 {
		t.Errorf("len(got) = %d, want 300", len(got))
	}
}

func TestFetchAllFiltered_StopsWhenServerIgnoresOffset(t *testing.T) {
	var reqs atomic.Int32
	ts := httptest.NewServer(countingHandler(&reqs, offsetIgnoringHandler("things")))
	defer ts.Close()

	got, hasMore, err := FetchAllFiltered[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0,
		func(testItem) bool { return true })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 100 || hasMore {
		t.Errorf("len(got) = %d, hasMore = %v; want 100, false", len(got), hasMore)
	}
	if n := reqs.Load(); n != 2 {
		t.Errorf("requests = %d, want 2", n)
	}
}

func TestFetchAllFiltered_PageCap(t *testing.T) {
	old := maxPages
	maxPages = 4
	t.Cleanup(func() { maxPages = old })

	var reqs atomic.Int32
	ts := httptest.NewServer(countingHandler(&reqs, endlessHandler("things")))
	defer ts.Close()

	if _, _, err := FetchAllFiltered[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0,
		func(testItem) bool { return false }); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n := reqs.Load(); n != 4 {
		t.Errorf("requests = %d, want 4 (page cap)", n)
	}
}

func TestFirstItemID(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{`[{"id":7,"name":"a"},{"id":8}]`, "7"},
		{`[{"id":"abc"}]`, `"abc"`},
		{`[{"title":"Wiki"}]`, ""},
		{`[]`, ""},
		{`null`, ""},
	}
	for _, tt := range tests {
		if got := firstItemID(json.RawMessage(tt.raw)); got != tt.want {
			t.Errorf("firstItemID(%s) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}
