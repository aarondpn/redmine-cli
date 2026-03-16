package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// paginatedResponse is the raw JSON wrapper for paginated Redmine responses.
type paginatedResponse struct {
	TotalCount int             `json:"total_count"`
	Offset     int             `json:"offset"`
	Limit      int             `json:"limit"`
	Items      json.RawMessage `json:"-"`
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
	totalCount := 0

	for {
		p := cloneParams(params)
		p.Set("limit", strconv.Itoa(pageSize))
		p.Set("offset", strconv.Itoa(offset))

		var raw map[string]json.RawMessage
		if err := c.Get(ctx, path, p, &raw); err != nil {
			return nil, 0, err
		}

		// Parse total_count
		if tc, ok := raw["total_count"]; ok {
			json.Unmarshal(tc, &totalCount)
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

		// Check if we have enough
		if maxResults > 0 && len(allItems) >= maxResults {
			allItems = allItems[:maxResults]
			break
		}

		// Check if there are more pages
		offset += pageSize
		if offset >= totalCount {
			break
		}
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

	for {
		p := cloneParams(params)
		p.Set("limit", strconv.Itoa(pageSize))
		p.Set("offset", strconv.Itoa(offset))

		var raw map[string]json.RawMessage
		if err := c.Get(ctx, path, p, &raw); err != nil {
			return nil, false, err
		}

		if tc, ok := raw["total_count"]; ok {
			json.Unmarshal(tc, &totalCount)
		}

		itemsRaw, ok := raw[key]
		if !ok {
			return nil, false, fmt.Errorf("response missing key %q", key)
		}

		var items []T
		if err := json.Unmarshal(itemsRaw, &items); err != nil {
			return nil, false, fmt.Errorf("decoding %s: %w", key, err)
		}

		for _, item := range items {
			if filter(item) {
				matched = append(matched, item)
				if maxResults > 0 && len(matched) >= maxResults {
					// We have enough, but there may be more on remaining pages.
					hasMore := offset+len(items) < totalCount || len(matched) > maxResults
					return matched[:maxResults], hasMore, nil
				}
			}
		}

		offset += pageSize
		if offset >= totalCount {
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
