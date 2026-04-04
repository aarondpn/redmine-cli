package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

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
