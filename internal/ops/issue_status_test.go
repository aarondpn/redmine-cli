package ops

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// statusServer is a fake Redmine that accepts issue PUTs with 204 (like the
// real one, even when it ignores status_id) and answers the read-back GET
// with a fixed issue body.
type statusServer struct {
	mu       sync.Mutex
	puts     int
	gets     int
	includes []string
	issueGet string // JSON body for GET /issues/42.json; empty means 500
}

func (s *statusServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/issue_statuses.json":
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issue_statuses":[{"id":1,"name":"New","is_closed":false},{"id":2,"name":"In Progress","is_closed":false},{"id":5,"name":"Closed","is_closed":true},{"id":6,"name":"Rejected","is_closed":true}]}`))
	case r.Method == http.MethodPut && r.URL.Path == "/issues/42.json":
		s.puts++
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && r.URL.Path == "/issues/42.json":
		s.gets++
		s.includes = append(s.includes, r.URL.Query().Get("include"))
		if s.issueGet == "" {
			http.Error(w, "boom", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(s.issueGet))
	default:
		http.Error(w, "unexpected "+r.Method+" "+r.URL.Path, http.StatusNotFound)
	}
}

const (
	issueStillNewWithAllowed = `{"issue":{"id":42,"status":{"id":1,"name":"New"},"allowed_statuses":[{"id":1,"name":"New","is_closed":false},{"id":2,"name":"In Progress","is_closed":false}]}}`
	issueStillNewOldServer   = `{"issue":{"id":42,"status":{"id":1,"name":"New"}}}`
	issueStillNewNoAllowed   = `{"issue":{"id":42,"status":{"id":1,"name":"New"},"allowed_statuses":[]}}`
	issueClosed              = `{"issue":{"id":42,"status":{"id":5,"name":"Closed"}}}`
)

func TestUpdateIssue_StatusApplied(t *testing.T) {
	srv := &statusServer{issueGet: issueClosed}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	status := 5
	res, err := UpdateIssue(context.Background(), client, UpdateIssueInput{ID: 42, StatusID: &status})
	if err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("unexpected warnings: %v", res.Warnings)
	}
	if srv.gets != 1 || srv.includes[0] != "allowed_statuses" {
		t.Errorf("read-back GETs = %d (include %v), want 1 with include=allowed_statuses", srv.gets, srv.includes)
	}
}

func TestUpdateIssue_NoStatusSkipsReadBack(t *testing.T) {
	srv := &statusServer{issueGet: issueClosed}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	subject := "renamed"
	if _, err := UpdateIssue(context.Background(), client, UpdateIssueInput{ID: 42, Subject: &subject}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if srv.gets != 0 {
		t.Errorf("GETs = %d, want 0 when no status change was requested", srv.gets)
	}
}

func TestUpdateIssue_StatusNotApplied(t *testing.T) {
	srv := &statusServer{issueGet: issueStillNewWithAllowed}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	status := 6
	note := "won't fix"
	_, err := UpdateIssue(context.Background(), client, UpdateIssueInput{ID: 42, StatusID: &status, Notes: &note})

	var notApplied *StatusNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want *StatusNotAppliedError", err)
	}
	if notApplied.Requested.ID != 6 || notApplied.Requested.Name != "Rejected" {
		t.Errorf("Requested = %+v, want Rejected (6) resolved via the statuses list", notApplied.Requested)
	}
	if notApplied.Current.ID != 1 || !notApplied.AllowedReported || len(notApplied.Allowed) != 2 {
		t.Errorf("unexpected error fields: %+v", notApplied)
	}
	if !notApplied.OtherChanges {
		t.Error("OtherChanges = false, want true (a note was sent)")
	}
	msg := err.Error()
	for _, want := range []string{
		`Issue #42: status was not changed to "Rejected" (ID 6); it is now "New" (ID 1)`,
		"workflow",
		"may already have been applied",
		"Allowed statuses: New (1), In Progress (2).",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
}

func TestUpdateIssue_StatusNotApplied_OldServer(t *testing.T) {
	srv := &statusServer{issueGet: issueStillNewOldServer}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	status := 5
	_, err := UpdateIssue(context.Background(), client, UpdateIssueInput{ID: 42, StatusID: &status})

	var notApplied *StatusNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want *StatusNotAppliedError", err)
	}
	if notApplied.AllowedReported {
		t.Error("AllowedReported = true for a server that did not return allowed_statuses")
	}
	if notApplied.OtherChanges {
		t.Error("OtherChanges = true for a status-only update")
	}
	msg := err.Error()
	if !strings.Contains(msg, "Redmine 5.0+") {
		t.Errorf("message should explain allowed statuses need Redmine 5.0+:\n%s", msg)
	}
	if strings.Contains(msg, "may already have been applied") {
		t.Errorf("status-only update should not mention other changes:\n%s", msg)
	}
}

func TestUpdateIssue_StatusNotApplied_NoTransitions(t *testing.T) {
	srv := &statusServer{issueGet: issueStillNewNoAllowed}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	status := 5
	_, err := UpdateIssue(context.Background(), client, UpdateIssueInput{ID: 42, StatusID: &status})
	if err == nil || !strings.Contains(err.Error(), "no allowed status transitions") {
		t.Fatalf("err = %v, want a message about no allowed transitions", err)
	}
}

func TestUpdateIssue_ReadBackFailureIsWarning(t *testing.T) {
	srv := &statusServer{}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	status := 5
	res, err := UpdateIssue(context.Background(), client, UpdateIssueInput{ID: 42, StatusID: &status})
	if err != nil {
		t.Fatalf("UpdateIssue: %v (a failed read-back must not turn an applied update into an error)", err)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "could not verify") {
		t.Fatalf("Warnings = %v, want one verification warning", res.Warnings)
	}
	if !strings.Contains(res.MCPMessage(), "Warning: could not verify") {
		t.Errorf("MCPMessage() = %q, want the warning appended", res.MCPMessage())
	}
}

func TestCloseIssue_StatusNotApplied(t *testing.T) {
	srv := &statusServer{issueGet: issueStillNewWithAllowed}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	_, err := CloseIssue(context.Background(), client, CloseIssueInput{ID: 42, Notes: "done"})
	var notApplied *StatusNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want *StatusNotAppliedError", err)
	}
	if notApplied.Requested.ID != 5 || notApplied.Requested.Name != "Closed" {
		t.Errorf("Requested = %+v, want Closed (5)", notApplied.Requested)
	}
	if !notApplied.OtherChanges {
		t.Error("OtherChanges = false, want true (a note was sent)")
	}
	if srv.puts != 1 {
		t.Errorf("PUTs = %d, want 1", srv.puts)
	}
}

func TestCloseIssue_StatusApplied(t *testing.T) {
	srv := &statusServer{issueGet: issueClosed}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	res, err := CloseIssue(context.Background(), client, CloseIssueInput{ID: 42})
	if err != nil {
		t.Fatalf("CloseIssue: %v", err)
	}
	if res.Message != "Closed issue #42" {
		t.Errorf("Message = %q", res.Message)
	}
}

func TestReopenIssue_StatusNotApplied(t *testing.T) {
	srv := &statusServer{issueGet: issueClosed}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	_, err := ReopenIssue(context.Background(), client, ReopenIssueInput{ID: 42})
	var notApplied *StatusNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want *StatusNotAppliedError", err)
	}
	if notApplied.Requested.ID != 1 || notApplied.Current.ID != 5 {
		t.Errorf("unexpected error fields: %+v", notApplied)
	}
}

// TestCloseIssue_AlreadyClosedOtherStatus pins that close succeeds when the
// issue already sits in a different closed status: Redmine keeps "Rejected"
// instead of moving it to the first closed status, which is fine for close.
func TestCloseIssue_AlreadyClosedOtherStatus(t *testing.T) {
	srv := &statusServer{issueGet: `{"issue":{"id":42,"status":{"id":6,"name":"Rejected"}}}`}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	res, err := CloseIssue(context.Background(), client, CloseIssueInput{ID: 42})
	if err != nil {
		t.Fatalf("CloseIssue: %v (an already-closed issue must not fail)", err)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], `already closed with status "Rejected"`) {
		t.Errorf("Warnings = %v, want an already-closed note", res.Warnings)
	}
}

// TestReopenIssue_AlreadyOpenOtherStatus is the reopen counterpart: an issue
// that is already "In Progress" counts as open.
func TestReopenIssue_AlreadyOpenOtherStatus(t *testing.T) {
	srv := &statusServer{issueGet: `{"issue":{"id":42,"status":{"id":2,"name":"In Progress"}}}`}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	res, err := ReopenIssue(context.Background(), client, ReopenIssueInput{ID: 42})
	if err != nil {
		t.Fatalf("ReopenIssue: %v (an already-open issue must not fail)", err)
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], `already open with status "In Progress"`) {
		t.Errorf("Warnings = %v, want an already-open note", res.Warnings)
	}
}

// TestUpdateIssue_OtherClosedStatusStillFails pins that the open/closed
// leniency is limited to close/reopen: update --status asks for one exact
// status.
func TestUpdateIssue_OtherClosedStatusStillFails(t *testing.T) {
	srv := &statusServer{issueGet: `{"issue":{"id":42,"status":{"id":6,"name":"Rejected"}}}`}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	status := 5
	_, err := UpdateIssue(context.Background(), client, UpdateIssueInput{ID: 42, StatusID: &status})
	var notApplied *StatusNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want *StatusNotAppliedError", err)
	}
}

func TestStatusNotAppliedError_SummaryAndDetails(t *testing.T) {
	e := &StatusNotAppliedError{
		IssueID:         1,
		Requested:       models.IDName{ID: 5, Name: "Closed"},
		Current:         models.IDName{ID: 1, Name: "New"},
		AllowedReported: true,
		OtherChanges:    true,
	}
	if strings.Contains(e.Summary(), "Allowed") || strings.Contains(e.Summary(), "already have been applied") {
		t.Errorf("Summary should not carry the details: %q", e.Summary())
	}
	details := e.Details()
	if len(details) != 2 || !strings.Contains(details[0], "no allowed status transitions") || !strings.Contains(details[1], "may already have been applied") {
		t.Errorf("Details = %q", details)
	}
}

// TestStatusNotAppliedError_SummaryPerTransition pins that the explanation
// matches the requested action: reopen never mentions the closing rules,
// while close and a plain status update (which may target a closed status)
// do.
func TestStatusNotAppliedError_SummaryPerTransition(t *testing.T) {
	tests := []struct {
		name        string
		transition  StatusTransition
		wantClosing bool
	}{
		{"close", StatusTransitionClose, true},
		{"reopen", StatusTransitionReopen, false},
		{"update", StatusTransitionUpdate, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &StatusNotAppliedError{
				IssueID:    1,
				Requested:  models.IDName{ID: 1, Name: "New"},
				Current:    models.IDName{ID: 5, Name: "Closed"},
				Transition: tt.transition,
			}
			summary := e.Summary()
			if !strings.Contains(summary, "workflow") {
				t.Errorf("summary missing workflow explanation: %q", summary)
			}
			if got := strings.Contains(summary, "open subtasks"); got != tt.wantClosing {
				t.Errorf("mentions closing rules = %v, want %v: %q", got, tt.wantClosing, summary)
			}
		})
	}
}

func TestReopenIssue_StatusNotApplied_SummaryIsReopenSpecific(t *testing.T) {
	srv := &statusServer{issueGet: issueClosed}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	_, err := ReopenIssue(context.Background(), client, ReopenIssueInput{ID: 42})
	var notApplied *StatusNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want *StatusNotAppliedError", err)
	}
	if notApplied.Transition != StatusTransitionReopen {
		t.Errorf("Transition = %v, want reopen", notApplied.Transition)
	}
	if strings.Contains(notApplied.Summary(), "close") {
		t.Errorf("reopen summary mentions closing: %q", notApplied.Summary())
	}
}

func TestCloseIssue_StatusNotApplied_TransitionIsClose(t *testing.T) {
	srv := &statusServer{issueGet: issueStillNewWithAllowed}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	_, err := CloseIssue(context.Background(), client, CloseIssueInput{ID: 42})
	var notApplied *StatusNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want *StatusNotAppliedError", err)
	}
	if notApplied.Transition != StatusTransitionClose {
		t.Errorf("Transition = %v, want close", notApplied.Transition)
	}
	if !strings.Contains(notApplied.Summary(), "refuses to close issues") {
		t.Errorf("close summary missing closing rules: %q", notApplied.Summary())
	}
}
