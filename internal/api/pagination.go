package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
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
	page := 1
	prevFirstID := ""

	for {
		p := cloneParams(params)
		p.Set("limit", strconv.Itoa(pageSize))
		p.Set("offset", strconv.Itoa(offset))

		c.debugLog.Printf("Pagination: fetching %s page %d (offset=%d, limit=%d)", path, page, offset, pageSize)

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

		// An empty page means there is nothing more to fetch, whatever
		// total_count claims.
		if len(items) == 0 {
			break
		}
		firstID := firstItemID(itemsRaw)
		if repeatedPage(prevFirstID, firstID) {
			c.debugLog.Printf("Pagination: page %d repeats the previous page (server ignored offset), stopping", page)
			break
		}
		prevFirstID = firstID

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

		// If the endpoint returned all items at once (doesn't support
		// pagination) or returned fewer than requested, stop.
		if len(items) >= totalCount || len(items) < pageSize {
			break
		}

		// Check if there are more pages
		offset += pageSize
		if offset >= totalCount {
			break
		}
		if page >= maxPages {
			c.debugLog.Printf("Pagination: reached the %d page safety cap, stopping", maxPages)
			break
		}
		page++
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
	page := 1
	prevFirstID := ""

	for {
		p := cloneParams(params)
		p.Set("limit", strconv.Itoa(pageSize))
		p.Set("offset", strconv.Itoa(offset))

		c.debugLog.Printf("Pagination (filtered): fetching %s page %d (offset=%d, limit=%d)", path, page, offset, pageSize)

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

		if len(items) == 0 {
			break
		}
		firstID := firstItemID(itemsRaw)
		if repeatedPage(prevFirstID, firstID) {
			c.debugLog.Printf("Pagination (filtered): page %d repeats the previous page (server ignored offset), stopping", page)
			break
		}
		prevFirstID = firstID

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

		c.debugLog.Printf("Pagination (filtered): page %d had %d items, %d matched so far (server total: %d)", page, len(items), len(matched), totalCount)

		// If the endpoint returned all items at once (doesn't support
		// pagination) or returned fewer than requested, stop.
		if len(items) >= totalCount || len(items) < pageSize {
			break
		}

		offset += pageSize
		if offset >= totalCount {
			break
		}
		if page >= maxPages {
			c.debugLog.Printf("Pagination (filtered): reached the %d page safety cap, stopping", maxPages)
			break
		}
		page++
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
