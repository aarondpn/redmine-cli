package issue

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestCmdIssueList_EmptyJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdList(f)
	cmd.SetArgs([]string{"--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if got := testutil.Stdout(f); got != "[]\n" {
		t.Fatalf("stdout = %q, want %q", got, "[]\n")
	}
	if got := testutil.Stderr(f); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestCmdIssueList_EmptyTableWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdList(f)
	cmd.SetArgs([]string{"--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	stderr := testutil.Stderr(f)
	if !strings.Contains(stderr, "No issues found") {
		t.Fatalf("stderr = %q, want warning about no issues found", stderr)
	}
}

func TestCmdIssueList_PaginationWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[{"id":1,"subject":"Test","tracker":{"id":1,"name":"Bug"},"status":{"id":1,"name":"New"},"priority":{"id":2,"name":"Normal"},"project":{"id":1,"name":"Demo"}}],"total_count":5}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdList(f)
	cmd.SetArgs([]string{"--output", "table", "--limit", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	stderr := testutil.Stderr(f)
	if !strings.Contains(stderr, "Showing 1 of 5 issues") {
		t.Fatalf("stderr = %q, want pagination warning", stderr)
	}
}

func TestCmdIssueList_NoPaginationWarningJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[{"id":1,"subject":"Test","tracker":{"id":1,"name":"Bug"},"status":{"id":1,"name":"New"},"priority":{"id":2,"name":"Normal"},"project":{"id":1,"name":"Demo"}}],"total_count":5}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdList(f)
	cmd.SetArgs([]string{"--output", "json", "--limit", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if got := testutil.Stderr(f); got != "" {
		t.Fatalf("stderr = %q, want empty (no warning for JSON)", got)
	}
}

func TestCmdIssueList_NoPaginationWarningLimitZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[{"id":1,"subject":"Test","tracker":{"id":1,"name":"Bug"},"status":{"id":1,"name":"New"},"priority":{"id":2,"name":"Normal"},"project":{"id":1,"name":"Demo"}}],"total_count":5}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdList(f)
	cmd.SetArgs([]string{"--output", "table", "--limit", "0"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	stderr := testutil.Stderr(f)
	if strings.Contains(stderr, "Showing") {
		t.Fatalf("stderr = %q, want no pagination warning when limit=0", stderr)
	}
}

func TestCmdIssueList_DefaultProjectFallback(t *testing.T) {
	var capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issues":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testutil.NewFactoryWithConfig(t, srv.URL, "default_project: mydefault\n")
	cmd := NewCmdList(f)
	cmd.SetArgs([]string{"--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	// The default project should be resolved and used in the API call
	if !strings.Contains(capturedPath, "issues") {
		t.Fatalf("expected issues endpoint to be called, got path %q", capturedPath)
	}
}

func TestCmdIssueList_ResolvesStatusNameToID(t *testing.T) {
	var issuesQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/issue_statuses.json":
			_, _ = w.Write([]byte(`{"issue_statuses":[{"id":3,"name":"Gelöst","is_closed":true},{"id":5,"name":"Closed","is_closed":true}]}`))
		case "/issues.json":
			issuesQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"issues":[],"total_count":0}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdList(f)
	cmd.SetArgs([]string{"--status", "Gelöst", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(issuesQuery, "status_id=3") {
		t.Fatalf("issues query = %q, want status_id=3", issuesQuery)
	}
	if strings.Contains(issuesQuery, "status_id=Gel") {
		t.Fatalf("issues query = %q, did not expect raw status name", issuesQuery)
	}
}

func TestCmdIssueList_ResolvesAssigneeNameToID(t *testing.T) {
	var issuesQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/users.json":
			_, _ = w.Write([]byte(`{"users":[{"id":7,"login":"jdoe","firstname":"John","lastname":"Doe","mail":"john@example.com","status":1}],"total_count":1}`))
		case "/issues.json":
			issuesQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"issues":[],"total_count":0}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdList(f)
	cmd.SetArgs([]string{"--assignee", "John Doe", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(issuesQuery, "assigned_to_id=7") {
		t.Fatalf("issues query = %q, want assigned_to_id=7", issuesQuery)
	}
	if strings.Contains(issuesQuery, "assigned_to_id=John") {
		t.Fatalf("issues query = %q, did not expect raw assignee name", issuesQuery)
	}
}

func TestCmdIssueList_IgnoresAssigneeNameWhenUserLookupForbidden(t *testing.T) {
	var issuesQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/users.json":
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`{"errors":["Forbidden"]}`))
		case "/issues.json":
			issuesQuery = r.URL.RawQuery
			_, _ = w.Write([]byte(`{"issues":[],"total_count":0}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdList(f)
	cmd.SetArgs([]string{"--assignee", "John Doe", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(issuesQuery, "assigned_to_id=") {
		t.Fatalf("issues query = %q, did not expect assigned_to_id filter", issuesQuery)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "ignoring the assignee filter") {
		t.Fatalf("stderr = %q, want assignee fallback warning", stderr)
	}
}
