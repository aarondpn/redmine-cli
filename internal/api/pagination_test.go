package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

type testItem struct {
	ID     int    `json:"id"`
	Status string `json:"status"`
}

// newTestClient creates a Client pointing at the given test server.
func newTestClient(ts *httptest.Server) *Client {
	return &Client{
		httpClient: ts.Client(),
		baseURL:    ts.URL,
		debugLog:   debug.New(nil),
	}
}

// paginatedHandler returns an http.Handler that serves items with standard
// Redmine pagination (respects offset and limit query params).
func paginatedHandler(key string, allItems []testItem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit == 0 {
			limit = len(allItems)
		}

		end := offset + limit
		if end > len(allItems) {
			end = len(allItems)
		}
		page := allItems[offset:end]

		resp := map[string]interface{}{
			key:           page,
			"total_count": len(allItems),
			"offset":      offset,
			"limit":       limit,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
}

// unpaginatedHandler returns an http.Handler that always returns ALL items,
// ignoring offset/limit (like the Redmine versions endpoint).
func unpaginatedHandler(key string, allItems []testItem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			key:           allItems,
			"total_count": len(allItems),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	})
}

func makeItems(n int, status string) []testItem {
	items := make([]testItem, n)
	for i := range items {
		items[i] = testItem{ID: i + 1, Status: status}
	}
	return items
}

func makeItemsMixed(n int) []testItem {
	items := make([]testItem, n)
	for i := range items {
		if i%2 == 0 {
			items[i] = testItem{ID: i + 1, Status: "open"}
		} else {
			items[i] = testItem{ID: i + 1, Status: "closed"}
		}
	}
	return items
}

// --- FetchAll tests ---

func TestFetchAll_SinglePage(t *testing.T) {
	items := makeItems(5, "open")
	ts := httptest.NewServer(paginatedHandler("things", items))
	defer ts.Close()

	got, total, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Errorf("total = %d, want 5", total)
	}
	if len(got) != 5 {
		t.Errorf("len(got) = %d, want 5", len(got))
	}
}

func TestFetchAll_MultiplePages(t *testing.T) {
	items := makeItems(250, "open")
	ts := httptest.NewServer(paginatedHandler("things", items))
	defer ts.Close()

	got, total, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 250 {
		t.Errorf("total = %d, want 250", total)
	}
	if len(got) != 250 {
		t.Errorf("len(got) = %d, want 250", len(got))
	}
	// Verify no duplicates
	seen := map[int]bool{}
	for _, item := range got {
		if seen[item.ID] {
			t.Errorf("duplicate item ID %d", item.ID)
		}
		seen[item.ID] = true
	}
}

func TestFetchAll_WithMaxResults(t *testing.T) {
	items := makeItems(250, "open")
	ts := httptest.NewServer(paginatedHandler("things", items))
	defer ts.Close()

	got, _, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 10 {
		t.Errorf("len(got) = %d, want 10", len(got))
	}
}

func TestFetchAll_UnpaginatedEndpoint(t *testing.T) {
	items := makeItems(50, "open")
	ts := httptest.NewServer(unpaginatedHandler("things", items))
	defer ts.Close()

	got, total, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 50 {
		t.Errorf("total = %d, want 50", total)
	}
	if len(got) != 50 {
		t.Errorf("len(got) = %d, want 50", len(got))
	}
}

func TestFetchAll_UnpaginatedEndpointLargerThanPageSize(t *testing.T) {
	// Simulates the versions bug: endpoint ignores pagination, returns all
	// 431 items every time, with total_count=431.
	items := makeItems(431, "open")
	var requestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		resp := map[string]interface{}{
			"things":      items,
			"total_count": len(items),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	got, total, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 431 {
		t.Errorf("total = %d, want 431", total)
	}
	if len(got) != 431 {
		t.Errorf("len(got) = %d, want 431", len(got))
	}
	if requestCount != 1 {
		t.Errorf("requestCount = %d, want 1 (should not re-fetch)", requestCount)
	}
}

// --- FetchAllFiltered tests ---

func TestFetchAllFiltered_SinglePage(t *testing.T) {
	items := makeItemsMixed(10) // 5 open, 5 closed
	ts := httptest.NewServer(paginatedHandler("things", items))
	defer ts.Close()

	got, hasMore, err := FetchAllFiltered[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0, func(i testItem) bool {
		return i.Status == "open"
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasMore {
		t.Error("hasMore = true, want false")
	}
	if len(got) != 5 {
		t.Errorf("len(got) = %d, want 5", len(got))
	}
}

func TestFetchAllFiltered_MultiplePages(t *testing.T) {
	items := makeItemsMixed(250) // 125 open, 125 closed
	ts := httptest.NewServer(paginatedHandler("things", items))
	defer ts.Close()

	got, _, err := FetchAllFiltered[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0, func(i testItem) bool {
		return i.Status == "open"
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 125 {
		t.Errorf("len(got) = %d, want 125", len(got))
	}
	// Verify no duplicates
	seen := map[int]bool{}
	for _, item := range got {
		if seen[item.ID] {
			t.Errorf("duplicate item ID %d", item.ID)
		}
		seen[item.ID] = true
	}
}

func TestFetchAllFiltered_WithMaxResults(t *testing.T) {
	items := makeItemsMixed(250) // 125 open
	ts := httptest.NewServer(paginatedHandler("things", items))
	defer ts.Close()

	got, hasMore, err := FetchAllFiltered[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 3, func(i testItem) bool {
		return i.Status == "open"
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len(got) = %d, want 3", len(got))
	}
	if !hasMore {
		t.Error("hasMore = false, want true")
	}
}

func TestFetchAllFiltered_UnpaginatedNoDuplicates(t *testing.T) {
	// The exact bug scenario: unpaginated endpoint with items > pageSize,
	// filtered by status. Should return each matching item exactly once.
	allItems := make([]testItem, 431)
	openCount := 0
	for i := range allItems {
		status := "closed"
		if i%70 == 0 {
			status = "open"
			openCount++
		}
		allItems[i] = testItem{ID: i + 1, Status: status}
	}

	var requestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		resp := map[string]interface{}{
			"things":      allItems,
			"total_count": len(allItems),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	got, hasMore, err := FetchAllFiltered[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0, func(i testItem) bool {
		return i.Status == "open"
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasMore {
		t.Error("hasMore = true, want false")
	}
	if len(got) != openCount {
		t.Errorf("len(got) = %d, want %d", len(got), openCount)
	}
	if requestCount != 1 {
		t.Errorf("requestCount = %d, want 1 (should not re-fetch)", requestCount)
	}

	// Verify no duplicates
	seen := map[int]bool{}
	for _, item := range got {
		if seen[item.ID] {
			t.Errorf("duplicate item ID %d", item.ID)
		}
		seen[item.ID] = true
	}
}

func TestFetchAllFiltered_NoMatches(t *testing.T) {
	items := makeItems(10, "closed")
	ts := httptest.NewServer(paginatedHandler("things", items))
	defer ts.Close()

	got, hasMore, err := FetchAllFiltered[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0, func(i testItem) bool {
		return i.Status == "open"
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hasMore {
		t.Error("hasMore = true, want false")
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

func TestFetchAll_RequestCount(t *testing.T) {
	// Verify correct number of API requests for paginated endpoints.
	items := makeItems(250, "open")
	var requestCount int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// Serve with proper pagination
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit == 0 {
			limit = 100
		}
		end := offset + limit
		if end > len(items) {
			end = len(items)
		}
		var page []testItem
		if offset < len(items) {
			page = items[offset:end]
		}
		resp := map[string]interface{}{
			"things":      page,
			"total_count": len(items),
			"offset":      offset,
			"limit":       limit,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	got, _, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 250 {
		t.Errorf("len(got) = %d, want 250", len(got))
	}
	// 250 items with pageSize 100 = 3 requests (100+100+50)
	if requestCount != 3 {
		t.Errorf("requestCount = %d, want 3", requestCount)
	}
}

func TestFetchAll_EmptyResponse(t *testing.T) {
	ts := httptest.NewServer(paginatedHandler("things", []testItem{}))
	defer ts.Close()

	got, total, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
	if len(got) != 0 {
		t.Errorf("len(got) = %d, want 0", len(got))
	}
}

func TestFetchAll_WithOffset(t *testing.T) {
	items := makeItems(250, "open")
	ts := httptest.NewServer(paginatedHandler("things", items))
	defer ts.Close()

	params := url.Values{}
	params.Set("offset", "50")
	got, total, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", params, "things", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 250 {
		t.Errorf("total = %d, want 250", total)
	}
	if len(got) != 200 {
		t.Errorf("len(got) = %d, want 200", len(got))
	}
	if got[0].ID != 51 {
		t.Errorf("first item ID = %d, want 51", got[0].ID)
	}
}

func TestFetchAll_WithOffsetAndMaxResults(t *testing.T) {
	items := makeItems(250, "open")
	ts := httptest.NewServer(paginatedHandler("things", items))
	defer ts.Close()

	params := url.Values{}
	params.Set("offset", "50")
	got, _, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", params, "things", 25)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 25 {
		t.Errorf("len(got) = %d, want 25", len(got))
	}
	if got[0].ID != 51 {
		t.Errorf("first item ID = %d, want 51", got[0].ID)
	}
	if got[24].ID != 75 {
		t.Errorf("last item ID = %d, want 75", got[24].ID)
	}
}

func TestFetchAll_UnpaginatedWithOffset(t *testing.T) {
	items := makeItems(50, "open")
	ts := httptest.NewServer(unpaginatedHandler("things", items))
	defer ts.Close()

	params := url.Values{}
	params.Set("offset", "10")
	got, total, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", params, "things", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 50 {
		t.Errorf("total = %d, want 50", total)
	}
	if len(got) != 40 {
		t.Errorf("len(got) = %d, want 40", len(got))
	}
	if got[0].ID != 11 {
		t.Errorf("first item ID = %d, want 11", got[0].ID)
	}
}

func TestFetchAll_UnpaginatedWithOffsetAndMaxResults(t *testing.T) {
	items := makeItems(50, "open")
	ts := httptest.NewServer(unpaginatedHandler("things", items))
	defer ts.Close()

	params := url.Values{}
	params.Set("offset", "10")
	got, _, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", params, "things", 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 5 {
		t.Errorf("len(got) = %d, want 5", len(got))
	}
	if got[0].ID != 11 {
		t.Errorf("first item ID = %d, want 11", got[0].ID)
	}
	if got[4].ID != 15 {
		t.Errorf("last item ID = %d, want 15", got[4].ID)
	}
}

func TestFetchAllFiltered_UnpaginatedWithMaxResults_HasMore(t *testing.T) {
	// Bug fix: when maxResults is reached mid-page on an unpaginated endpoint,
	// hasMore should be true if there are unscanned items remaining on the page.
	allItems := make([]testItem, 50)
	openCount := 0
	for i := range allItems {
		if i%5 == 0 {
			allItems[i] = testItem{ID: i + 1, Status: "open"}
			openCount++
		} else {
			allItems[i] = testItem{ID: i + 1, Status: "closed"}
		}
	}
	// openCount = 10 (items at index 0,5,10,...,45)

	ts := httptest.NewServer(unpaginatedHandler("things", allItems))
	defer ts.Close()

	// Request only 3 matching items — more open items exist beyond that.
	got, hasMore, err := FetchAllFiltered[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 3, func(i testItem) bool {
		return i.Status == "open"
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len(got) = %d, want 3", len(got))
	}
	if !hasMore {
		t.Error("hasMore = false, want true (there are 10 open items but we only asked for 3)")
	}
}

func TestFetchAllFiltered_SinglePageExactMatch_NoHasMore(t *testing.T) {
	// When maxResults exactly matches the total number of filtered items on a
	// single page and we finish scanning the last item, hasMore should be false.
	allItems := []testItem{
		{ID: 1, Status: "closed"},
		{ID: 2, Status: "closed"},
		{ID: 3, Status: "open"}, // match #1
		{ID: 4, Status: "closed"},
		{ID: 5, Status: "open"}, // match #2 — last item
	}

	ts := httptest.NewServer(paginatedHandler("things", allItems))
	defer ts.Close()

	got, hasMore, err := FetchAllFiltered[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 2, func(i testItem) bool {
		return i.Status == "open"
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len(got) = %d, want 2", len(got))
	}
	// The last match was the last item on the only page, so no more.
	if hasMore {
		t.Error("hasMore = true, want false (all items scanned, all matches consumed)")
	}
}

func TestFetchAll_MissingKey(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"total_count": 0}`)
	}))
	defer ts.Close()

	_, _, err := FetchAll[testItem](context.Background(), newTestClient(ts), "/things.json", nil, "things", 0)
	if err == nil {
		t.Fatal("expected error for missing key, got nil")
	}
}
