package version

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// --- list ---

func TestVersionList_RequiresProject(t *testing.T) {
	f := testutil.NewFactory(t, "http://unused")
	cmd := newCmdVersionList(f)
	cmd.SetArgs([]string{"--output", "json"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no project is specified")
	}
	if !strings.Contains(err.Error(), "project is required") {
		t.Errorf("error = %q, want 'project is required'", err.Error())
	}
}

func TestVersionList_EmptyJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"versions":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionList(f)
	cmd.SetArgs([]string{"--project", "1", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Stdout(f); got != "[]\n" {
		t.Fatalf("stdout = %q, want %q", got, "[]\n")
	}
}

func TestVersionList_Table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"versions":[{"id":1,"project":{"id":1,"name":"Demo"},"name":"v1.0","status":"open","due_date":"2025-07-01","sharing":"none","description":"First release","created_on":"","updated_on":""}],"total_count":1}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionList(f)
	cmd.SetArgs([]string{"--project", "1", "--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"v1.0", "open", "2025-07-01"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout missing %q:\n%s", want, stdout)
		}
	}
}

func TestVersionList_OpenFilter(t *testing.T) {
	// The versions endpoint returns all versions; --open triggers client-side
	// filtering via ListFiltered. Return a mix of statuses and verify only
	// "open" versions appear in the output.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"versions":[
			{"id":1,"project":{"id":1,"name":"Demo"},"name":"v1.0","status":"closed","due_date":"2025-01-01","sharing":"none","description":"","created_on":"","updated_on":""},
			{"id":2,"project":{"id":1,"name":"Demo"},"name":"v2.0","status":"open","due_date":"2025-07-01","sharing":"none","description":"","created_on":"","updated_on":""},
			{"id":3,"project":{"id":1,"name":"Demo"},"name":"v3.0","status":"locked","due_date":"2025-12-01","sharing":"none","description":"","created_on":"","updated_on":""}
		],"total_count":3}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionList(f)
	cmd.SetArgs([]string{"--project", "1", "--open", "--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	if !strings.Contains(stdout, "v2.0") {
		t.Errorf("expected open version v2.0 in output:\n%s", stdout)
	}
	if strings.Contains(stdout, "v1.0") {
		t.Errorf("closed version v1.0 should be filtered out:\n%s", stdout)
	}
	if strings.Contains(stdout, "v3.0") {
		t.Errorf("locked version v3.0 should be filtered out:\n%s", stdout)
	}
}

func TestVersionList_FilteredOffsetTrim(t *testing.T) {
	// When --open and --offset are combined, the command fetches all matching
	// versions then trims client-side. Verify that offset=1 drops the first
	// matched version.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"versions":[
			{"id":1,"project":{"id":1,"name":"Demo"},"name":"v1.0","status":"open","due_date":"","sharing":"none","description":"","created_on":"","updated_on":""},
			{"id":2,"project":{"id":1,"name":"Demo"},"name":"v2.0","status":"open","due_date":"","sharing":"none","description":"","created_on":"","updated_on":""},
			{"id":3,"project":{"id":1,"name":"Demo"},"name":"v3.0","status":"closed","due_date":"","sharing":"none","description":"","created_on":"","updated_on":""}
		],"total_count":3}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionList(f)
	cmd.SetArgs([]string{"--project", "1", "--open", "--offset", "1", "--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	// v1.0 should be skipped by offset, v2.0 should be shown, v3.0 is closed
	if strings.Contains(stdout, "v1.0") {
		t.Errorf("v1.0 should be trimmed by offset:\n%s", stdout)
	}
	if !strings.Contains(stdout, "v2.0") {
		t.Errorf("expected v2.0 in output:\n%s", stdout)
	}
}

func TestVersionList_UnfilteredPaginationWarning(t *testing.T) {
	// Without status filtering, the standard pagination path is used.
	// Verify the warning when total > limit+offset.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"versions":[
			{"id":1,"project":{"id":1,"name":"Demo"},"name":"v1.0","status":"open","due_date":"","sharing":"none","description":"","created_on":"","updated_on":""}
		],"total_count":5}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionList(f)
	cmd.SetArgs([]string{"--project", "1", "--limit", "1", "--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stderr := testutil.Stderr(f)
	if !strings.Contains(stderr, "Showing 1 of 5 versions") {
		t.Errorf("stderr = %q, want pagination warning", stderr)
	}
}

func TestVersionList_ClosedFilter_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"versions":[
			{"id":1,"project":{"id":1,"name":"Demo"},"name":"v1.0","status":"closed","due_date":"","sharing":"none","description":"","created_on":"","updated_on":""},
			{"id":2,"project":{"id":1,"name":"Demo"},"name":"v2.0","status":"open","due_date":"","sharing":"none","description":"","created_on":"","updated_on":""}
		],"total_count":2}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionList(f)
	cmd.SetArgs([]string{"--project", "1", "--closed", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	if !strings.Contains(stdout, "v1.0") {
		t.Errorf("expected closed version v1.0 in JSON output:\n%s", stdout)
	}
	if strings.Contains(stdout, "v2.0") {
		t.Errorf("open version v2.0 should be filtered out of JSON output:\n%s", stdout)
	}
}

// --- get ---

func TestVersionGet_Detail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/versions/1.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":{"id":1,"project":{"id":1,"name":"Demo"},"name":"v1.0","status":"open","due_date":"2025-07-01","sharing":"none","description":"First release","created_on":"2025-01-01","updated_on":"2025-06-01"}}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionGet(f)
	cmd.SetArgs([]string{"1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"v1.0", "Demo", "open", "2025-07-01"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("detail output missing %q:\n%s", want, stdout)
		}
	}
}
