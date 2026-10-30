package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// GetWithAllowedStatuses has to tell three server answers apart: include
// ignored (Redmine < 5.0, key absent), no transition allowed ([]) and a
// populated list.
func TestIssueService_GetWithAllowedStatuses(t *testing.T) {
	tests := []struct {
		name         string
		body         string
		wantReported bool
		wantAllowed  int
	}{
		{"include ignored by older server", `{"issue":{"id":7,"subject":"s","status":{"id":1,"name":"New"}}}`, false, 0},
		{"empty list", `{"issue":{"id":7,"subject":"s","allowed_statuses":[]}}`, true, 0},
		{"populated list", `{"issue":{"id":7,"subject":"s","status":{"id":1,"name":"New","is_closed":false},"allowed_statuses":[{"id":1,"name":"New","is_closed":false},{"id":5,"name":"Closed","is_closed":true}]}}`, true, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotInclude string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotInclude = r.URL.Query().Get("include")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(tt.body))
			}))
			defer ts.Close()

			c := newTestClient(ts)
			c.Issues = &IssueService{client: c}

			issue, allowed, reported, err := c.Issues.GetWithAllowedStatuses(context.Background(), 7)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotInclude != "allowed_statuses" {
				t.Errorf("include = %q, want allowed_statuses", gotInclude)
			}
			if reported != tt.wantReported {
				t.Errorf("reported = %v, want %v", reported, tt.wantReported)
			}
			if len(allowed) != tt.wantAllowed {
				t.Errorf("allowed = %d entries, want %d", len(allowed), tt.wantAllowed)
			}
			if tt.wantReported && allowed == nil {
				t.Error("a reported list must be non-nil")
			}
			if !tt.wantReported && allowed != nil {
				t.Errorf("unreported list must be nil, got %v", allowed)
			}
			if issue == nil || issue.ID != 7 {
				t.Errorf("issue = %+v", issue)
			}
		})
	}
}
