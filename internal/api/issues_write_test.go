package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// captureBody starts a server that records the method, path and JSON body of
// the last request and answers with status and response.
func captureBody(t *testing.T, status int, response string) (*Client, *string, *string, *map[string]any) {
	t.Helper()
	var method, path string
	var body map[string]any
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		raw, _ := io.ReadAll(r.Body)
		body = nil
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("request body is not JSON: %s", raw)
			}
		}
		if response != "" {
			w.Header().Set("Content-Type", "application/json")
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(response))
	}))
	t.Cleanup(ts.Close)
	c := newTestClient(ts)
	c.Issues = &IssueService{client: c}
	return c, &method, &path, &body
}

// The copy parameters live at the top level of the body; Redmine ignores
// them inside the issue hash. The flags must be the string "1", which is
// what Redmine compares link_copy and copy_attachments against.
func TestIssueService_Copy_TopLevelParams(t *testing.T) {
	c, method, path, body := captureBody(t, http.StatusCreated, `{"issue":{"id":99,"subject":"Copy"}}`)

	assignee := 0
	issue, err := c.Issues.Copy(context.Background(), models.IssueCopy{
		CopyFrom:        12,
		Link:            true,
		CopyAttachments: true,
		CopySubtasks:    true,
		Issue:           models.IssueCopyAttributes{ProjectID: 3, Subject: "Copy", AssignedToID: &assignee},
	})
	if err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if issue.ID != 99 {
		t.Fatalf("issue = %+v", issue)
	}
	if *method != http.MethodPost || *path != "/issues.json" {
		t.Fatalf("request = %s %s", *method, *path)
	}
	want := map[string]any{
		"copy_from":        float64(12),
		"project_id":       float64(3),
		"link_copy":        "1",
		"copy_attachments": "1",
		"copy_subtasks":    "1",
		"issue":            map[string]any{"project_id": float64(3), "subject": "Copy", "assigned_to_id": ""},
	}
	if !reflect.DeepEqual(*body, want) {
		t.Fatalf("body = %#v\nwant  %#v", *body, want)
	}
}

// Unrequested copy options are omitted, leaving the server setting in
// charge, and empty overrides are not sent.
func TestIssueService_Copy_OmitsUnsetOptions(t *testing.T) {
	c, _, _, body := captureBody(t, http.StatusCreated, `{"issue":{"id":99}}`)

	if _, err := c.Issues.Copy(context.Background(), models.IssueCopy{
		CopyFrom: 12,
		Issue:    models.IssueCopyAttributes{ProjectID: 3},
	}); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	want := map[string]any{
		"copy_from":  float64(12),
		"project_id": float64(3),
		"issue":      map[string]any{"project_id": float64(3)},
	}
	if !reflect.DeepEqual(*body, want) {
		t.Fatalf("body = %#v\nwant  %#v", *body, want)
	}
}

// A non-zero assignee goes out as a number.
func TestIssueService_Copy_AssigneeID(t *testing.T) {
	c, _, _, body := captureBody(t, http.StatusCreated, `{"issue":{"id":99}}`)

	assignee := 7
	if _, err := c.Issues.Copy(context.Background(), models.IssueCopy{
		CopyFrom: 12,
		Issue:    models.IssueCopyAttributes{ProjectID: 3, AssignedToID: &assignee},
	}); err != nil {
		t.Fatalf("Copy: %v", err)
	}
	if got := (*body)["issue"].(map[string]any)["assigned_to_id"]; got != float64(7) {
		t.Fatalf("assigned_to_id = %#v, want 7", got)
	}
}

func TestIssueService_AddWatchers_OneRequest(t *testing.T) {
	c, method, path, body := captureBody(t, http.StatusNoContent, "")

	if err := c.Issues.AddWatchers(context.Background(), 5, []int{3, 7}); err != nil {
		t.Fatalf("AddWatchers: %v", err)
	}
	if *method != http.MethodPost || *path != "/issues/5/watchers.json" {
		t.Fatalf("request = %s %s", *method, *path)
	}
	want := map[string]any{"watcher": map[string]any{"user_ids": []any{float64(3), float64(7)}}}
	if !reflect.DeepEqual(*body, want) {
		t.Fatalf("body = %#v\nwant  %#v", *body, want)
	}
}

func TestIssueService_Update_MoveAndDeleteAttachments(t *testing.T) {
	c, method, path, body := captureBody(t, http.StatusNoContent, "")

	project := 4
	if err := c.Issues.Update(context.Background(), 5, models.IssueUpdate{
		ProjectID:            &project,
		DeletedAttachmentIDs: []int{8, 9},
	}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if *method != http.MethodPut || *path != "/issues/5.json" {
		t.Fatalf("request = %s %s", *method, *path)
	}
	want := map[string]any{"issue": map[string]any{
		"project_id":             float64(4),
		"deleted_attachment_ids": []any{float64(8), float64(9)},
	}}
	if !reflect.DeepEqual(*body, want) {
		t.Fatalf("body = %#v\nwant  %#v", *body, want)
	}
}

func TestIssueService_Create_DoneRatio(t *testing.T) {
	c, _, _, body := captureBody(t, http.StatusCreated, `{"issue":{"id":1}}`)

	zero := 0
	if _, err := c.Issues.Create(context.Background(), models.IssueCreate{ProjectID: 1, Subject: "s", DoneRatio: &zero}); err != nil {
		t.Fatalf("Create: %v", err)
	}
	issue := (*body)["issue"].(map[string]any)
	if got, ok := issue["done_ratio"]; !ok || got != float64(0) {
		t.Fatalf("done_ratio = %v (present %v), want 0", got, ok)
	}
}
