package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// Redmine omits children for leaf issues and relations when there are none,
// even when they were requested. Get must still report them as fetched.
func TestIssueService_Get_NormalizesRequestedCollections(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issue":{"id":7,"subject":"Leaf","journals":[]}}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Issues = &IssueService{client: c}

	issue, err := c.Issues.Get(context.Background(), 7, []string{"journals", "children", "relations"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if issue.Journals == nil || issue.Children == nil || issue.Relations == nil || issue.CustomFields == nil {
		t.Fatalf("requested collections must be non-nil: journals=%v children=%v relations=%v custom_fields=%v",
			issue.Journals, issue.Children, issue.Relations, issue.CustomFields)
	}
	if issue.Attachments != nil || issue.Watchers != nil {
		t.Fatalf("unrequested collections must stay nil: attachments=%v watchers=%v", issue.Attachments, issue.Watchers)
	}

	plain, err := c.Issues.Get(context.Background(), 7, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plain.Children != nil || plain.Relations != nil {
		t.Fatalf("children/relations must stay nil without include: %v/%v", plain.Children, plain.Relations)
	}
}

// The list endpoint ignores include=children, so List must not claim an
// empty children collection for it.
func TestIssueService_List_DoesNotForceUnsupportedIncludes(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[{"id":1,"relations":[]}],"total_count":1}`))
	}))
	defer ts.Close()

	c := newTestClient(ts)
	c.Issues = &IssueService{client: c}

	issues, _, err := c.Issues.List(context.Background(), models.IssueFilter{Includes: []string{"relations", "children"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("got %d issues, want 1", len(issues))
	}
	if issues[0].Relations == nil {
		t.Error("relations sent as [] must stay non-nil")
	}
	if issues[0].Children != nil {
		t.Errorf("children = %v, want nil (not supported by the list endpoint)", issues[0].Children)
	}
	if issues[0].CustomFields == nil {
		t.Error("custom_fields must be non-nil (always part of the issue response)")
	}
}
