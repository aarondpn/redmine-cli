package ops

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
)

// issueWriteServer answers issue PUTs with putStatus/putBody, issue GETs with
// an issue in project getProject, and records every request.
type issueWriteServer struct {
	mu         sync.Mutex
	putStatus  int
	putBody    string
	getProject int
	requests   []string
	bodies     []map[string]any
}

func (s *issueWriteServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	var body map[string]any
	_ = json.Unmarshal(raw, &body)
	s.mu.Lock()
	s.requests = append(s.requests, r.Method+" "+r.URL.Path)
	s.bodies = append(s.bodies, body)
	s.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodPut:
		status := s.putStatus
		if status == 0 {
			status = http.StatusNoContent
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(s.putBody))
	case http.MethodGet:
		_ = json.NewEncoder(w).Encode(map[string]any{"issue": map[string]any{
			"id": 5, "project": map[string]any{"id": s.getProject, "name": "Old"}, "status": map[string]any{"id": 1, "name": "New"},
		}})
	case http.MethodPost:
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"issue":{"id":77,"subject":"Copy"}}`))
	}
}

func intPtr(v int) *int { return &v }

func TestUpdateIssue_MoveApplied(t *testing.T) {
	srv := &issueWriteServer{getProject: 4}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	res, err := UpdateIssue(context.Background(), client, UpdateIssueInput{ID: 5, ProjectID: intPtr(4)})
	if err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if len(res.Warnings) != 0 {
		t.Fatalf("warnings = %v", res.Warnings)
	}
	if got := strings.Join(srv.requests, ","); got != "PUT /issues/5.json,GET /issues/5.json" {
		t.Fatalf("requests = %s", got)
	}
	if issue := srv.bodies[0]["issue"].(map[string]any); issue["project_id"] != float64(4) {
		t.Fatalf("PUT body = %v", srv.bodies[0])
	}
}

func TestUpdateIssue_MoveIgnored(t *testing.T) {
	srv := &issueWriteServer{getProject: 1}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	notes := "moving"
	_, err := UpdateIssue(context.Background(), client, UpdateIssueInput{ID: 5, ProjectID: intPtr(4), Notes: &notes})
	var notApplied *ProjectNotAppliedError
	if !errors.As(err, &notApplied) {
		t.Fatalf("err = %v, want ProjectNotAppliedError", err)
	}
	if notApplied.RequestedID != 4 || notApplied.Current.ID != 1 || !notApplied.OtherChanges {
		t.Fatalf("error = %+v", notApplied)
	}
	if !strings.Contains(notApplied.Summary(), "Add issues") || len(notApplied.Details()) != 1 {
		t.Fatalf("summary = %q details = %v", notApplied.Summary(), notApplied.Details())
	}
}

// Only a move triggers the project read-back; attachment removal is sent
// as-is.
func TestUpdateIssue_DeletedAttachmentsNoReadBack(t *testing.T) {
	srv := &issueWriteServer{}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	if _, err := UpdateIssue(context.Background(), client, UpdateIssueInput{ID: 5, DeletedAttachmentIDs: []int{8}}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	if len(srv.requests) != 1 {
		t.Fatalf("requests = %v, want only the PUT", srv.requests)
	}
	issue := srv.bodies[0]["issue"].(map[string]any)
	if ids, _ := issue["deleted_attachment_ids"].([]any); len(ids) != 1 || ids[0] != float64(8) {
		t.Fatalf("deleted_attachment_ids = %v", issue["deleted_attachment_ids"])
	}
}

func TestCopyIssue_DefaultsToSourceProject(t *testing.T) {
	srv := &issueWriteServer{getProject: 6}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	issue, err := CopyIssue(context.Background(), client, CopyIssueInput{ID: 5, LinkCopy: true})
	if err != nil {
		t.Fatalf("CopyIssue: %v", err)
	}
	if issue.ID != 77 {
		t.Fatalf("issue = %+v", issue)
	}
	if got := strings.Join(srv.requests, ","); got != "GET /issues/5.json,POST /issues.json" {
		t.Fatalf("requests = %s", got)
	}
	body := srv.bodies[1]
	if body["copy_from"] != float64(5) || body["link_copy"] != "1" || body["project_id"] != float64(6) {
		t.Fatalf("POST body = %v", body)
	}
	if _, ok := body["copy_attachments"]; ok {
		t.Fatalf("copy_attachments sent without being requested: %v", body)
	}
}

func TestCopyIssue_GivenProjectSkipsLookup(t *testing.T) {
	srv := &issueWriteServer{}
	client, closeTS := newTestAPIClient(t, srv)
	defer closeTS()

	if _, err := CopyIssue(context.Background(), client, CopyIssueInput{ID: 5, ProjectID: 9, Subject: "New", CopySubtasks: true}); err != nil {
		t.Fatalf("CopyIssue: %v", err)
	}
	if len(srv.requests) != 1 {
		t.Fatalf("requests = %v, want only the POST", srv.requests)
	}
	issue := srv.bodies[0]["issue"].(map[string]any)
	if issue["project_id"] != float64(9) || issue["subject"] != "New" || srv.bodies[0]["copy_subtasks"] != "1" {
		t.Fatalf("POST body = %v", srv.bodies[0])
	}
}

func TestCopyIssue_RejectsBadID(t *testing.T) {
	if _, err := CopyIssue(context.Background(), nil, CopyIssueInput{}); err == nil {
		t.Fatal("expected an error for id 0")
	}
}

func TestAddIssueWatchers(t *testing.T) {
	for _, tc := range []struct {
		name string
		ids  []int
		want map[string]any
	}{
		{"one user keeps user_id", []int{3}, map[string]any{"user_id": float64(3)}},
		{"several users in one request", []int{3, 4}, map[string]any{"watcher": map[string]any{"user_ids": []any{float64(3), float64(4)}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := &issueWriteServer{}
			client, closeTS := newTestAPIClient(t, srv)
			defer closeTS()

			if err := AddIssueWatchers(context.Background(), client, 5, tc.ids); err != nil {
				t.Fatalf("AddIssueWatchers: %v", err)
			}
			if len(srv.requests) != 1 || srv.requests[0] != "POST /issues/5/watchers.json" {
				t.Fatalf("requests = %v", srv.requests)
			}
			got, _ := json.Marshal(srv.bodies[0])
			want, _ := json.Marshal(tc.want)
			if string(got) != string(want) {
				t.Fatalf("body = %s, want %s", got, want)
			}
		})
	}
	if err := AddIssueWatchers(context.Background(), nil, 5, []int{0}); err == nil {
		t.Fatal("expected an error for user ID 0")
	}
}

func TestCreateIssue_DoneRatio(t *testing.T) {
	var body map[string]any
	client, closeTS := newTestAPIClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"issue":{"id":1}}`))
	}))
	defer closeTS()

	if _, err := CreateIssue(context.Background(), client, CreateIssueInput{ProjectID: 1, Subject: "s", DoneRatio: intPtr(40)}); err != nil {
		t.Fatalf("CreateIssue: %v", err)
	}
	if issue := body["issue"].(map[string]any); issue["done_ratio"] != float64(40) {
		t.Fatalf("body = %v", body)
	}
}
