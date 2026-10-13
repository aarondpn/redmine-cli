package ops

import (
	"context"
	"net/http"
	"testing"
)

// update_issue with assigned_to_id 0 must clear the assignee. Redmine
// rejects 0 ("Assignee is invalid"), so the wire value is an empty string.
func TestUpdateIssue_ZeroAssigneeUnassigns(t *testing.T) {
	cap := &captureIssueUpdateHandler{}
	client, closeTS := newTestAPIClient(t, cap)
	defer closeTS()

	zero := 0
	if _, err := UpdateIssue(context.Background(), client, UpdateIssueInput{ID: 42, AssignedToID: &zero}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	issue := cap.lastIssue(t)
	if v, ok := issue["assigned_to_id"]; !ok || v != "" {
		t.Fatalf("assigned_to_id = %#v (present=%v), want empty string", v, ok)
	}
}

func TestListIssues_NoneAssigneeMapsToUnassignedFilter(t *testing.T) {
	var gotQuery string
	client, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("assigned_to_id")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[],"total_count":0}`))
	}))
	defer closeTS()

	if _, err := ListIssues(context.Background(), client, ListIssuesInput{AssignedToID: "none"}); err != nil {
		t.Fatalf("ListIssues: %v", err)
	}
	if gotQuery != "!*" {
		t.Fatalf("assigned_to_id = %q, want %q", gotQuery, "!*")
	}
}
