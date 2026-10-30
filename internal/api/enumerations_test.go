package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEnumerationService_ListPaths(t *testing.T) {
	cases := []struct {
		name string
		path string
		body string
		call func(c *Client) (int, string, bool, error)
	}{
		{
			name: "priorities",
			path: "/enumerations/issue_priorities.json",
			body: `{"issue_priorities":[{"id":2,"name":"Normal","is_default":true,"active":true}]}`,
			call: func(c *Client) (int, string, bool, error) {
				items, err := c.Enumerations.IssuePriorities(context.Background())
				if err != nil || len(items) != 1 {
					return 0, "", false, err
				}
				return items[0].ID, items[0].Name, items[0].CustomFields != nil, nil
			},
		},
		{
			name: "activities",
			path: "/enumerations/time_entry_activities.json",
			body: `{"time_entry_activities":[{"id":9,"name":"Design","is_default":false,"active":true}]}`,
			call: func(c *Client) (int, string, bool, error) {
				items, err := c.Enumerations.TimeEntryActivities(context.Background())
				if err != nil || len(items) != 1 {
					return 0, "", false, err
				}
				return items[0].ID, items[0].Name, items[0].CustomFields != nil, nil
			},
		},
		{
			name: "document categories",
			path: "/enumerations/document_categories.json",
			body: `{"document_categories":[{"id":3,"name":"Technical documentation","is_default":false,"active":true}]}`,
			call: func(c *Client) (int, string, bool, error) {
				items, err := c.Enumerations.DocumentCategories(context.Background())
				if err != nil || len(items) != 1 {
					return 0, "", false, err
				}
				return items[0].ID, items[0].Name, items[0].CustomFields != nil, nil
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotPath string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tc.body))
			}))
			defer ts.Close()
			c := newTestClient(ts)
			c.Enumerations = &EnumerationService{client: c}

			id, name, normalized, err := tc.call(c)
			if err != nil {
				t.Fatal(err)
			}
			if gotPath != tc.path {
				t.Errorf("path = %q, want %q", gotPath, tc.path)
			}
			if id == 0 || name == "" {
				t.Errorf("decoded id=%d name=%q", id, name)
			}
			if !normalized {
				t.Error("custom_fields not normalized to an empty slice")
			}
		})
	}
}

func TestEnumerationService_MissingKeyIsEmptyList(t *testing.T) {
	c := newFieldsTestClient(t, `{}`, nil)
	items, err := c.Enumerations.DocumentCategories(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if items == nil || len(items) != 0 {
		t.Errorf("items = %#v, want empty non-nil slice", items)
	}
}
