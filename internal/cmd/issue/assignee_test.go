package issue

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// assigneeServer is a fake Redmine for assignee resolution tests. Issue 5
// lives in project 7 ("demo"), whose memberships contain a user and a group.
// /users.json is forbidden, mimicking a non-admin API key.
type assigneeServer struct {
	t *testing.T

	mu       sync.Mutex
	requests []string
	putBody  map[string]any
	postBody map[string]any
	query    map[string]string
}

func (s *assigneeServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	s.requests = append(s.requests, r.Method+" "+r.URL.Path)
	s.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/issues/5.json":
		_, _ = w.Write([]byte(`{"issue":{"id":5,"subject":"s","project":{"id":7,"name":"Demo"},"tracker":{"id":1,"name":"Bug"},"status":{"id":1,"name":"New"},"priority":{"id":2,"name":"Normal"}}}`))
	case r.Method == http.MethodPut && r.URL.Path == "/issues/5.json":
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &s.putBody)
		w.WriteHeader(http.StatusNoContent)
	case r.Method == http.MethodGet && (r.URL.Path == "/projects/7.json" || r.URL.Path == "/projects/demo.json"):
		_, _ = w.Write([]byte(`{"project":{"id":7,"identifier":"demo","name":"Demo"}}`))
	case r.Method == http.MethodGet && (r.URL.Path == "/projects/7/memberships.json" || r.URL.Path == "/projects/demo/memberships.json"):
		_, _ = w.Write([]byte(`{"memberships":[
			{"id":1,"project":{"id":7,"name":"Demo"},"user":{"id":3,"name":"Alice Smith"},"roles":[{"id":4,"name":"Developer"}]},
			{"id":2,"project":{"id":7,"name":"Demo"},"group":{"id":12,"name":"Backend Team"},"roles":[{"id":4,"name":"Developer"}]}
		],"total_count":2}`))
	case r.URL.Path == "/users.json":
		w.WriteHeader(http.StatusForbidden)
	case r.Method == http.MethodGet && r.URL.Path == "/issues.json":
		s.mu.Lock()
		s.query = map[string]string{"assigned_to_id": r.URL.Query().Get("assigned_to_id")}
		s.mu.Unlock()
		_, _ = w.Write([]byte(`{"issues":[],"total_count":0}`))
	case r.Method == http.MethodPost && r.URL.Path == "/issues.json":
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &s.postBody)
		_, _ = w.Write([]byte(`{"issue":{"id":9,"subject":"s","project":{"id":7,"name":"Demo"},"tracker":{"id":1,"name":"Bug"},"status":{"id":1,"name":"New"},"priority":{"id":2,"name":"Normal"}}}`))
	default:
		s.t.Errorf("unexpected request %s %s", r.Method, r.URL.String())
		w.WriteHeader(http.StatusNotFound)
	}
}

func (s *assigneeServer) start() string {
	srv := httptest.NewServer(s)
	s.t.Cleanup(srv.Close)
	return srv.URL
}

func (s *assigneeServer) issuePayload(t *testing.T) map[string]any {
	t.Helper()
	issue, ok := s.putBody["issue"].(map[string]any)
	if !ok {
		t.Fatalf("PUT body missing issue wrapper: %v (requests: %v)", s.putBody, s.requests)
	}
	return issue
}

func (s *assigneeServer) hit(req string) bool {
	for _, r := range s.requests {
		if r == req {
			return true
		}
	}
	return false
}

func TestCmdIssueUpdate_AssigneeNoneClearsWithoutLookups(t *testing.T) {
	s := &assigneeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdUpdate(f)
	cmd.SetArgs([]string{"5", "--assignee", "none"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	issue := s.issuePayload(t)
	if v, ok := issue["assigned_to_id"]; !ok || v != "" {
		t.Fatalf("assigned_to_id = %#v (present=%v), want empty string", v, ok)
	}
	if len(s.requests) != 1 {
		t.Fatalf("requests = %v, want only the PUT", s.requests)
	}
}

func TestCmdIssueUpdate_AssigneeGroupNameUsesIssueProjectMemberships(t *testing.T) {
	s := &assigneeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdUpdate(f)
	cmd.SetArgs([]string{"5", "--assignee", "backend team"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if got := s.issuePayload(t)["assigned_to_id"]; got != float64(12) {
		t.Fatalf("assigned_to_id = %#v, want 12", got)
	}
	if !s.hit("GET /issues/5.json") || !s.hit("GET /projects/7/memberships.json") {
		t.Fatalf("expected issue + memberships lookups, got %v", s.requests)
	}
	if s.hit("GET /users.json") {
		t.Fatalf("member match must not consult the admin-only users endpoint: %v", s.requests)
	}
}

func TestCmdIssueUpdate_NumericAssigneeSkipsIssueFetch(t *testing.T) {
	s := &assigneeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdUpdate(f)
	cmd.SetArgs([]string{"5", "--assignee", "3"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := s.issuePayload(t)["assigned_to_id"]; got != float64(3) {
		t.Fatalf("assigned_to_id = %#v, want 3", got)
	}
	if len(s.requests) != 1 {
		t.Fatalf("requests = %v, want only the PUT", s.requests)
	}
}

func TestCmdIssueUpdate_UnknownAssigneeListsMembers(t *testing.T) {
	s := &assigneeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdUpdate(f)
	cmd.SetArgs([]string{"5", "--assignee", "jdoe"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"resolving assignee", "Alice Smith [user] (ID: 3)", "Backend Team [group] (ID: 12)"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
	if s.putBody != nil {
		t.Fatalf("no update should be sent on resolution failure")
	}
}

func TestCmdIssueAssign_None(t *testing.T) {
	s := &assigneeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdAssign(f)
	cmd.SetArgs([]string{"5", "none"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if v, ok := s.issuePayload(t)["assigned_to_id"]; !ok || v != "" {
		t.Fatalf("assigned_to_id = %#v (present=%v), want empty string", v, ok)
	}
	if out := testutil.Stdout(f) + testutil.Stderr(f); !strings.Contains(out, "nassigned issue #5") {
		t.Fatalf("output = %q, want unassigned message", out)
	}
}

func TestCmdIssueAssign_GroupName(t *testing.T) {
	s := &assigneeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdAssign(f)
	cmd.SetArgs([]string{"5", "Backend Team"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := s.issuePayload(t)["assigned_to_id"]; got != float64(12) {
		t.Fatalf("assigned_to_id = %#v, want 12", got)
	}
}

func TestCmdIssueList_AssigneeNoneFiltersUnassigned(t *testing.T) {
	s := &assigneeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdList(f)
	cmd.SetArgs([]string{"--assignee", "none", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := s.query["assigned_to_id"]; got != "!*" {
		t.Fatalf("assigned_to_id = %q, want %q", got, "!*")
	}
}

func TestCmdIssueList_AssigneeGroupNameWithProject(t *testing.T) {
	s := &assigneeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdList(f)
	cmd.SetArgs([]string{"--project", "demo", "--assignee", "Backend Team", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if got := s.query["assigned_to_id"]; got != "12" {
		t.Fatalf("assigned_to_id = %q, want 12", got)
	}
	if stderr := testutil.Stderr(f); strings.Contains(stderr, "ignoring") {
		t.Fatalf("filter must not be dropped: %q", stderr)
	}
}

func TestCmdIssueCreate_AssigneeNoneLeavesUnassigned(t *testing.T) {
	s := &assigneeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdCreate(f)
	cmd.SetArgs([]string{"--project", "7", "--subject", "s", "--assignee", "none", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	issue, ok := s.postBody["issue"].(map[string]any)
	if !ok {
		t.Fatalf("POST body missing issue wrapper: %v", s.postBody)
	}
	if _, ok := issue["assigned_to_id"]; ok {
		t.Fatalf("assigned_to_id should be omitted for none: %v", issue)
	}
	for _, r := range s.requests {
		if strings.HasSuffix(r, "/memberships.json") || r == "GET /users.json" {
			t.Fatalf("none must not trigger a principal lookup: %v", s.requests)
		}
	}
}

func TestCmdIssueCreate_WatcherAndAssigneeNamesResolveViaMemberships(t *testing.T) {
	s := &assigneeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdCreate(f)
	cmd.SetArgs([]string{"--project", "7", "--subject", "s", "--assignee", "Alice Smith", "--watcher", "Backend Team", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	issue, ok := s.postBody["issue"].(map[string]any)
	if !ok {
		t.Fatalf("POST body missing issue wrapper: %v", s.postBody)
	}
	if issue["assigned_to_id"] != float64(3) {
		t.Fatalf("assigned_to_id = %#v, want 3", issue["assigned_to_id"])
	}
	watchers, _ := issue["watcher_user_ids"].([]any)
	if len(watchers) != 1 || watchers[0] != float64(12) {
		t.Fatalf("watcher_user_ids = %#v, want [12]", issue["watcher_user_ids"])
	}
	fetches := 0
	for _, r := range s.requests {
		if strings.HasSuffix(r, "/memberships.json") {
			fetches++
		}
	}
	if fetches != 1 {
		t.Fatalf("memberships fetched %d times, want 1 shared lookup: %v", fetches, s.requests)
	}
}

// A literal 0 must not silently clear the assignee: "none" is the documented
// way to unassign, and Redmine has no principal with ID 0.
func TestCmdIssueUpdate_AssigneeZeroOrNegativeRejected(t *testing.T) {
	for _, val := range []string{"0", "-3"} {
		t.Run(val, func(t *testing.T) {
			s := &assigneeServer{t: t}
			f := testutil.NewFactory(t, s.start())

			cmd := NewCmdUpdate(f)
			cmd.SetArgs([]string{"5", "--assignee", val})
			err := cmd.Execute()
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), "use 'none' to unassign") {
				t.Errorf("error = %q, want hint to use 'none'", err)
			}
			if len(s.requests) != 0 {
				t.Fatalf("requests = %v, want none", s.requests)
			}
		})
	}
}

func TestCmdIssueCreate_AssigneeZeroRejected(t *testing.T) {
	s := &assigneeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdCreate(f)
	cmd.SetArgs([]string{"--project", "7", "--subject", "s", "--assignee", "0", "--output", "json"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "use 'none' to unassign") {
		t.Fatalf("err = %v, want rejection with hint to use 'none'", err)
	}
	if s.postBody != nil {
		t.Fatalf("no issue should be created: %v", s.postBody)
	}
}

func TestCmdIssueAssign_ZeroPointsToNone(t *testing.T) {
	s := &assigneeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdAssign(f)
	cmd.SetArgs([]string{"5", "0"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "redmine issues assign 5 none") {
		t.Errorf("error = %q, want CLI hint 'redmine issues assign 5 none'", err)
	}
	if strings.Contains(err.Error(), "update_issue") {
		t.Errorf("error = %q, must not point CLI users at the MCP tool", err)
	}
	if len(s.requests) != 0 {
		t.Fatalf("requests = %v, want none", s.requests)
	}
}

func TestCmdIssueAssign_UnknownNameSaysAssignee(t *testing.T) {
	s := &assigneeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdAssign(f)
	cmd.SetArgs([]string{"5", "Nobody Here"})
	err := cmd.Execute()
	if err == nil || !strings.HasPrefix(err.Error(), "resolving assignee: ") {
		t.Fatalf("err = %v, want 'resolving assignee:' prefix", err)
	}
}

func TestCmdWatchers_UnknownNameSaysWatcher(t *testing.T) {
	for _, sub := range []string{"add", "remove"} {
		t.Run(sub, func(t *testing.T) {
			s := &assigneeServer{t: t}
			f := testutil.NewFactory(t, s.start())

			cmd := NewCmdWatchers(f)
			cmd.SetArgs([]string{sub, "5", "Nobody Here"})
			err := cmd.Execute()
			if err == nil || !strings.HasPrefix(err.Error(), "resolving watcher: ") {
				t.Fatalf("err = %v, want 'resolving watcher:' prefix", err)
			}
		})
	}
}

func TestCmdIssueList_AuthorGroupNameExplainsGroupsCannotAuthor(t *testing.T) {
	s := &assigneeServer{t: t}
	f := testutil.NewFactory(t, s.start())

	cmd := NewCmdList(f)
	cmd.SetArgs([]string{"--project", "demo", "--author", "Backend Team", "--output", "json"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"resolving author", `"Backend Team" is a group (ID: 12)`, "cannot be issue authors"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
	if s.hit("GET /issues.json") {
		t.Fatalf("issues must not be listed with an unresolved author: %v", s.requests)
	}
}
