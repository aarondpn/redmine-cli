package issue

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// writeServer serves the endpoints the issue write commands touch and
// records every request with its JSON body. The issue lives in project
// issueProject; GET /projects/other.json resolves to project 4.
type writeServer struct {
	t            *testing.T
	mu           sync.Mutex
	issueProject int
	putStatus    int
	putBody      string
	requests     []string
	bodies       map[string]map[string]any
}

func (s *writeServer) start() string {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		key := r.Method + " " + r.URL.Path
		s.mu.Lock()
		s.requests = append(s.requests, key)
		if len(raw) > 0 {
			var body map[string]any
			if err := json.Unmarshal(raw, &body); err != nil {
				s.t.Errorf("%s: body is not JSON: %s", key, raw)
			}
			if s.bodies == nil {
				s.bodies = map[string]map[string]any{}
			}
			s.bodies[key] = body
		}
		s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch key {
		case "GET /projects/other.json":
			_, _ = w.Write([]byte(`{"project":{"id":4,"identifier":"other","name":"Other"}}`))
		case "GET /projects/17.json":
			_, _ = w.Write([]byte(`{"project":{"id":17,"identifier":"demo","name":"Demo"}}`))
		case "GET /issues/5.json":
			project := s.issueProject
			if project == 0 {
				project = 1
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"issue": map[string]any{
				"id": 5, "subject": "Source", "project": map[string]any{"id": project, "name": "P"},
			}})
		case "PUT /issues/5.json":
			status := s.putStatus
			if status == 0 {
				status = http.StatusNoContent
			}
			w.WriteHeader(status)
			_, _ = w.Write([]byte(s.putBody))
		case "POST /issues.json":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"issue":{"id":9,"subject":"Source","project":{"id":4,"name":"Other"},"tracker":{"id":1,"name":"Bug"},"status":{"id":1,"name":"New"},"priority":{"id":2,"name":"Normal"}}}`))
		case "POST /issues/5/watchers.json":
			w.WriteHeader(http.StatusNoContent)
		// Names resolve only in the target project "other"; a lookup in
		// the issue's own project 1 is an unexpected request.
		case "GET /projects/other/issue_categories.json":
			_, _ = w.Write([]byte(`{"issue_categories":[{"id":31,"name":"Backend","project":{"id":4,"name":"Other"}}],"total_count":1}`))
		case "GET /projects/other/versions.json":
			_, _ = w.Write([]byte(`{"versions":[{"id":41,"name":"v2","status":"open","project":{"id":4,"name":"Other"}}],"total_count":1}`))
		case "GET /projects/other/memberships.json":
			_, _ = w.Write([]byte(`{"memberships":[{"id":1,"project":{"id":4,"name":"Other"},"user":{"id":51,"name":"Bob Target"},"roles":[{"id":4,"name":"Developer"}]}],"total_count":1}`))
		case "GET /users.json":
			w.WriteHeader(http.StatusForbidden)
		default:
			s.t.Errorf("unexpected request %s", key)
			http.NotFound(w, r)
		}
	}))
	s.t.Cleanup(ts.Close)
	return ts.URL
}

func (s *writeServer) issueBody(key string) map[string]any {
	s.t.Helper()
	body, ok := s.bodies[key]
	if !ok {
		s.t.Fatalf("no body recorded for %s (requests: %v)", key, s.requests)
	}
	issue, _ := body["issue"].(map[string]any)
	return issue
}

func TestCmdIssueUpdate_MoveProject(t *testing.T) {
	s := &writeServer{t: t, issueProject: 4}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdUpdate(f)
	cmd.SetArgs([]string{"5", "--project", "other"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := s.issueBody("PUT /issues/5.json")["project_id"]; got != float64(4) {
		t.Fatalf("project_id = %v, want 4", got)
	}
	// The move is read back.
	if got := strings.Join(s.requests, ","); got != "GET /projects/other.json,PUT /issues/5.json,GET /issues/5.json" {
		t.Fatalf("requests = %s", got)
	}
}

// With --project, category, version and assignee names are resolved in the
// target project, not in the issue's current one.
func TestCmdIssueUpdate_MoveResolvesNamesInTargetProject(t *testing.T) {
	s := &writeServer{t: t, issueProject: 4}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdUpdate(f)
	cmd.SetArgs([]string{"5", "--project", "other", "--category", "Backend", "--version", "v2", "--assignee", "Bob Target"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	issue := s.issueBody("PUT /issues/5.json")
	for k, want := range map[string]float64{"project_id": 4, "category_id": 31, "fixed_version_id": 41, "assigned_to_id": 51} {
		if issue[k] != want {
			t.Errorf("%s = %v, want %v", k, issue[k], want)
		}
	}
}

func TestCmdIssueUpdate_MoveIgnoredIsCodedError(t *testing.T) {
	s := &writeServer{t: t, issueProject: 1}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdUpdate(f)
	cmd.SetArgs([]string{"5", "--project", "other"})
	err := cmd.Execute()
	var coded *cmdutil.CodedError
	if !errors.As(err, &coded) || coded.Code != output.ErrCodeProjectNotApplied {
		t.Fatalf("err = %v, want code project_not_applied", err)
	}
}

func TestCmdIssueUpdate_RemoveAttachment(t *testing.T) {
	t.Run("force", func(t *testing.T) {
		s := &writeServer{t: t}
		f := testutil.NewFactory(t, s.start())

		cmd := NewCmdUpdate(f)
		cmd.SetArgs([]string{"5", "--remove-attachment", "8", "--remove-attachment", "9", "--force"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		ids, _ := s.issueBody("PUT /issues/5.json")["deleted_attachment_ids"].([]any)
		if len(ids) != 2 || ids[0] != float64(8) || ids[1] != float64(9) {
			t.Fatalf("deleted_attachment_ids = %v", ids)
		}
	})
	t.Run("confirmed", func(t *testing.T) {
		s := &writeServer{t: t}
		f := testutil.NewFactory(t, s.start())
		f.IOStreams.In = bytes.NewBufferString("y\n")

		cmd := NewCmdUpdate(f)
		cmd.SetArgs([]string{"5", "--remove-attachment", "8"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		if !strings.Contains(testutil.Stderr(f), "Remove attachment(s) 8 from issue #5?") {
			t.Fatalf("prompt missing: %q", testutil.Stderr(f))
		}
		s.issueBody("PUT /issues/5.json")
	})
	t.Run("declined", func(t *testing.T) {
		s := &writeServer{t: t}
		f := testutil.NewFactory(t, s.start())

		cmd := NewCmdUpdate(f)
		cmd.SetArgs([]string{"5", "--remove-attachment", "8", "--subject", "x"})
		if err := cmd.Execute(); err != nil {
			t.Fatalf("execute: %v", err)
		}
		if len(s.requests) != 0 {
			t.Fatalf("requests = %v, want none after declining", s.requests)
		}
	})
	t.Run("stdin file needs force", func(t *testing.T) {
		s := &writeServer{t: t}
		f := testutil.NewFactory(t, s.start())

		cmd := NewCmdUpdate(f)
		cmd.SetArgs([]string{"5", "--remove-attachment", "8", "--note-file", "-"})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), "add --force") {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("invalid id", func(t *testing.T) {
		s := &writeServer{t: t}
		f := testutil.NewFactory(t, s.start())

		cmd := NewCmdUpdate(f)
		cmd.SetArgs([]string{"5", "--remove-attachment", "0", "--force"})
		if err := cmd.Execute(); err == nil {
			t.Fatal("expected an error for attachment ID 0")
		}
	})
}

func TestCmdIssueUpdate_TooManyAttachDescriptions(t *testing.T) {
	s := &writeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdUpdate(f)
	cmd.SetArgs([]string{"5", "--attach-description", "orphan"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--attach-description given 1 time(s) but --attach only 0:") {
		t.Fatalf("err = %v", err)
	}
	if len(s.requests) != 0 {
		t.Fatalf("requests = %v, want none", s.requests)
	}
}

func TestCmdIssueCreate_DoneRatio(t *testing.T) {
	var posted map[string]any
	srv := newCreateIssueServer(t, &posted)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdCreate(f)
	cmd.SetArgs([]string{"--project", "17", "--subject", "test", "--done-ratio", "0", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	issue := posted["issue"].(map[string]any)
	if got, ok := issue["done_ratio"]; !ok || got != float64(0) {
		t.Fatalf("done_ratio = %v (present %v), want 0", got, ok)
	}
}

func TestCmdIssueCreate_NoDoneRatioByDefault(t *testing.T) {
	var posted map[string]any
	srv := newCreateIssueServer(t, &posted)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdCreate(f)
	cmd.SetArgs([]string{"--project", "17", "--subject", "test", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, ok := posted["issue"].(map[string]any)["done_ratio"]; ok {
		t.Fatalf("done_ratio sent without --done-ratio: %v", posted)
	}
}

func TestCmdIssueCopy(t *testing.T) {
	s := &writeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdCopy(f)
	cmd.SetArgs([]string{"5", "--project", "other", "--link", "--with-attachments", "--with-subtasks", "--subject", "Copy", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	body := s.bodies["POST /issues.json"]
	want := map[string]any{
		"copy_from": float64(5), "project_id": float64(4),
		"link_copy": "1", "copy_attachments": "1", "copy_subtasks": "1",
	}
	for k, v := range want {
		if body[k] != v {
			t.Errorf("%s = %v, want %v", k, body[k], v)
		}
	}
	issue := body["issue"].(map[string]any)
	if issue["project_id"] != float64(4) || issue["subject"] != "Copy" {
		t.Fatalf("issue = %v", issue)
	}
	var out struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &out); err != nil || out.ID != 9 {
		t.Fatalf("stdout = %q", testutil.Stdout(f))
	}
}

// Without --project the copy goes to the source issue's project, which
// Redmine requires in the request.
func TestCmdIssueCopy_DefaultProject(t *testing.T) {
	s := &writeServer{t: t, issueProject: 6}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdCopy(f)
	cmd.SetArgs([]string{"5"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	body := s.bodies["POST /issues.json"]
	if body["project_id"] != float64(6) || body["issue"].(map[string]any)["project_id"] != float64(6) {
		t.Fatalf("body = %v", body)
	}
	for _, k := range []string{"link_copy", "copy_attachments", "copy_subtasks"} {
		if _, ok := body[k]; ok {
			t.Errorf("%s sent without its flag", k)
		}
	}
	if !strings.Contains(testutil.Stdout(f)+testutil.Stderr(f), "Copied issue #5 to #9") {
		t.Fatalf("output = %q / %q", testutil.Stdout(f), testutil.Stderr(f))
	}
}

func TestCmdWatchersAdd_SeveralUsersOneRequest(t *testing.T) {
	s := &writeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdWatchers(f)
	cmd.SetArgs([]string{"add", "5", "3", "7"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if len(s.requests) != 1 {
		t.Fatalf("requests = %v, want one POST", s.requests)
	}
	got, _ := json.Marshal(s.bodies["POST /issues/5/watchers.json"])
	if string(got) != `{"watcher":{"user_ids":[3,7]}}` {
		t.Fatalf("body = %s", got)
	}
}

func TestCmdIssueCopy_AssigneeResolvedInTargetProject(t *testing.T) {
	s := &writeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdCopy(f)
	cmd.SetArgs([]string{"5", "--project", "other", "--assignee", "Bob Target"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := s.issueBody("POST /issues.json")["assigned_to_id"]; got != float64(51) {
		t.Fatalf("assigned_to_id = %v, want 51", got)
	}
}

// Redmine clears the copied assignee only for a blank value; 0 fails with
// "Assignee is invalid".
func TestCmdIssueCopy_AssigneeNoneSendsBlank(t *testing.T) {
	s := &writeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdCopy(f)
	cmd.SetArgs([]string{"5", "--assignee", "none"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got, ok := s.issueBody("POST /issues.json")["assigned_to_id"]; !ok || got != "" {
		t.Fatalf("assigned_to_id = %#v (present %v), want \"\"", got, ok)
	}
}
