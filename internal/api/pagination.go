package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

// maxPages is an absolute cap on the pages a single FetchAll or
// FetchAllFiltered call requests (a million items at the default page size),
// so a server that keeps reporting a larger total_count cannot keep the
// client paging forever.
var maxPages = 10000

// firstItemID returns the raw "id" of the first element of a JSON array, or
// "" when the array is empty or its items carry no id. It is used to detect
// servers that ignore the offset parameter and return the same page again.
func firstItemID(itemsRaw json.RawMessage) string {
	dec := json.NewDecoder(bytes.NewReader(itemsRaw))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('[') || !dec.More() {
		return ""
	}
	var first struct {
		ID json.RawMessage `json:"id"`
	}
	if err := dec.Decode(&first); err != nil {
		return ""
	}
	return string(first.ID)
}

// repeatedPage reports whether a page starts with the same item as the
// previous page, which means the server ignored the requested offset.
func repeatedPage(prevFirstID, firstID string) bool {
	return prevFirstID != "" && firstID == prevFirstID
}

// pageGuard holds the defenses FetchAll and FetchAllFiltered share against a
// server that misreports pagination: paging stops on an empty page, on a
// page that repeats the previous one and after maxPages pages, and a page
// longer than requested is cut to the page size, so neither a lying
// total_count nor ignored offset or limit parameters can make the client
// page forever or multiply the results.
type pageGuard struct {
	log         *debug.Logger
	name        string // log prefix
	pageSize    int
	page        int
	prevFirstID string
}

func newPageGuard(log *debug.Logger, name string, pageSize int) *pageGuard {
	return &pageGuard{log: log, name: name, pageSize: pageSize, page: 1}
}

// acceptPage vets a decoded page. It returns ok == false when paging must
// stop without using the page: it is empty, or it starts with the same item
// as the previous page (the server ignored offset). Otherwise it returns the
// items to keep. A page with more items than requested, while total_count
// says more pages follow, comes from a server that ignored limit; only the
// first pageSize items are kept, since the next page starts pageSize items
// further on and would repeat the rest. A page that holds the whole
// collection (at least total_count items, as endpoints without pagination
// return) is kept in full.
func acceptPage[T any](g *pageGuard, items []T, itemsRaw json.RawMessage, totalCount int) ([]T, bool) {
	// An empty page means there is nothing more to fetch, whatever
	// total_count claims.
	if len(items) == 0 {
		return nil, false
	}
	firstID := firstItemID(itemsRaw)
	if repeatedPage(g.prevFirstID, firstID) {
		g.log.Printf("%s: page %d repeats the previous page (server ignored offset), stopping", g.name, g.page)
		return nil, false
	}
	g.prevFirstID = firstID
	if len(items) > g.pageSize && len(items) < totalCount {
		g.log.Printf("%s: page %d has %d items, more than the requested %d (server ignored limit); keeping the first %d", g.name, g.page, len(items), g.pageSize, g.pageSize)
		items = items[:g.pageSize]
	}
	return items, true
}

// next reports whether to fetch another page after one that kept n items,
// advancing *offset and the page counter when it does.
func (g *pageGuard) next(n, totalCount int, offset *int) bool {
	// If the endpoint returned all items at once (doesn't support
	// pagination) or returned fewer than requested, stop.
	if n >= totalCount || n < g.pageSize {
		return false
	}
	*offset += g.pageSize
	if *offset >= totalCount {
		return false
	}
	if g.page >= maxPages {
		g.log.Printf("%s: reached the %d page safety cap, stopping", g.name, maxPages)
		return false
	}
	g.page++
	return true
}

// FetchAll retrieves all pages of a resource. The key parameter is the JSON
// wrapper key (e.g., "issues", "projects"). If maxResults is 0, all results
// are fetched.
func FetchAll[T any](ctx context.Context, c *Client, path string, params url.Values, key string, maxResults int) ([]T, int, error) {
	if params == nil {
		params = url.Values{}
	}

	pageSize := 100
	if maxResults > 0 && maxResults < pageSize {
		pageSize = maxResults
	}

	var allItems []T
	offset := 0
	if v := params.Get("offset"); v != "" {
		offset, _ = strconv.Atoi(v)
	}
	startOffset := offset
	totalCount := 0
	guard := newPageGuard(c.debugLog, "Pagination", pageSize)

	for {
		p := cloneParams(params)
		p.Set("limit", strconv.Itoa(pageSize))
		p.Set("offset", strconv.Itoa(offset))

		c.debugLog.Printf("Pagination: fetching %s page %d (offset=%d, limit=%d)", path, guard.page, offset, pageSize)

		var raw map[string]json.RawMessage
		if err := c.Get(ctx, path, p, &raw); err != nil {
			return nil, 0, err
		}

		// Parse total_count
		if tc, ok := raw["total_count"]; ok {
			if err := json.Unmarshal(tc, &totalCount); err != nil {
				return nil, 0, fmt.Errorf("decoding total_count: %w", err)
			}
		}

		// Parse items
		itemsRaw, ok := raw[key]
		if !ok {
			return nil, 0, fmt.Errorf("response missing key %q", key)
		}

		var items []T
		if err := json.Unmarshal(itemsRaw, &items); err != nil {
			return nil, 0, fmt.Errorf("decoding %s: %w", key, err)
		}

		items, ok = acceptPage(guard, items, itemsRaw, totalCount)
		if !ok {
			break
		}

		allItems = append(allItems, items...)

		c.debugLog.Printf("Pagination: received %d items (total so far: %d, server total: %d)", len(items), len(allItems), totalCount)

		// For unpaginated endpoints that ignore offset, apply it client-side.
		if startOffset > 0 && len(items) >= totalCount {
			if startOffset < len(allItems) {
				allItems = allItems[startOffset:]
			} else {
				allItems = nil
			}
		}

		// Check if we have enough
		if maxResults > 0 && len(allItems) >= maxResults {
			allItems = allItems[:maxResults]
			break
		}

		if !guard.next(len(items), totalCount, &offset) {
			break
		}
	}

	// Apply maxResults after any offset adjustment
	if maxResults > 0 && len(allItems) > maxResults {
		allItems = allItems[:maxResults]
	}

	return allItems, totalCount, nil
}

// FetchAllFiltered retrieves pages of a resource, keeping only items that pass
// the filter. If maxResults is 0, all matching items are fetched. Returns the
// matched items and whether more matches may exist beyond what was collected.
func FetchAllFiltered[T any](ctx context.Context, c *Client, path string, params url.Values, key string, maxResults int, filter func(T) bool) ([]T, bool, error) {
	if params == nil {
		params = url.Values{}
	}

	pageSize := 100
	var matched []T
	offset := 0
	totalCount := 0
	guard := newPageGuard(c.debugLog, "Pagination (filtered)", pageSize)

	for {
		p := cloneParams(params)
		p.Set("limit", strconv.Itoa(pageSize))
		p.Set("offset", strconv.Itoa(offset))

		c.debugLog.Printf("Pagination (filtered): fetching %s page %d (offset=%d, limit=%d)", path, guard.page, offset, pageSize)

		var raw map[string]json.RawMessage
		if err := c.Get(ctx, path, p, &raw); err != nil {
			return nil, false, err
		}

		if tc, ok := raw["total_count"]; ok {
			if err := json.Unmarshal(tc, &totalCount); err != nil {
				return nil, false, fmt.Errorf("decoding total_count: %w", err)
			}
		}

		itemsRaw, ok := raw[key]
		if !ok {
			return nil, false, fmt.Errorf("response missing key %q", key)
		}

		var items []T
		if err := json.Unmarshal(itemsRaw, &items); err != nil {
			return nil, false, fmt.Errorf("decoding %s: %w", key, err)
		}

		items, ok = acceptPage(guard, items, itemsRaw, totalCount)
		if !ok {
			break
		}

		for i, item := range items {
			if filter(item) {
				matched = append(matched, item)
				if maxResults > 0 && len(matched) >= maxResults {
					// We have enough, but there may be more on the rest
					// of this page or on subsequent pages.
					moreItemsOnPage := i < len(items)-1
					morePagesExist := offset+len(items) < totalCount
					hasMore := moreItemsOnPage || morePagesExist
					c.debugLog.Printf("Pagination (filtered): matched %d items (limit reached)", len(matched))
					return matched[:maxResults], hasMore, nil
				}
			}
		}

		c.debugLog.Printf("Pagination (filtered): page %d had %d items, %d matched so far (server total: %d)", guard.page, len(items), len(matched), totalCount)

		if !guard.next(len(items), totalCount, &offset) {
			break
		}
	}

	return matched, false, nil
}

func cloneParams(p url.Values) url.Values {
	clone := url.Values{}
	for k, v := range p {
		clone[k] = append([]string{}, v...)
	}
	return clone
}
