package version

import (
	"encoding/json"
	"io"
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

// --- create ---

func TestVersionCreate_SendsBody(t *testing.T) {
	var capturedMethod, capturedPath string
	var capturedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/projects/demo.json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"project":{"id":1,"name":"Demo","identifier":"demo","description":"","status":1,"is_public":true,"created_on":"","updated_on":""}}`))
			return
		}
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &capturedBody); err != nil {
			t.Fatalf("bad JSON body: %v\n%s", err, body)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"version":{"id":7,"project":{"id":1,"name":"Demo"},"name":"v1.2","status":"open","due_date":"2026-06-30","sharing":"none","description":"Release","wiki_page_title":"ReleaseNotes","created_on":"","updated_on":""}}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionCreate(f)
	cmd.SetArgs([]string{
		"--project", "demo",
		"--name", "v1.2",
		"--status", "open",
		"--sharing", "none",
		"--due-date", "2026-06-30",
		"--description", "Release",
		"--wiki-page-title", "ReleaseNotes",
		"--output", "json",
	})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if capturedMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", capturedMethod)
	}
	if capturedPath != "/projects/demo/versions.json" {
		t.Errorf("path = %s, want /projects/demo/versions.json", capturedPath)
	}
	version, ok := capturedBody["version"].(map[string]any)
	if !ok {
		t.Fatalf("body missing version key: %+v", capturedBody)
	}
	wantFields := map[string]string{
		"name":            "v1.2",
		"status":          "open",
		"sharing":         "none",
		"due_date":        "2026-06-30",
		"description":     "Release",
		"wiki_page_title": "ReleaseNotes",
	}
	for field, want := range wantFields {
		if got, _ := version[field].(string); got != want {
			t.Errorf("version[%s] = %q, want %q", field, got, want)
		}
	}
	if !strings.Contains(testutil.Stdout(f), `"id": 7`) {
		t.Errorf("stdout = %q, want created version JSON", testutil.Stdout(f))
	}
}

// --- update ---

func TestVersionUpdate_SendsChangedFieldsOnly(t *testing.T) {
	var capturedMethod, capturedPath string
	var capturedBody map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &capturedBody); err != nil {
			t.Fatalf("bad JSON body: %v\n%s", err, body)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionUpdate(f)
	cmd.SetArgs([]string{"7", "--name", "v1.2.1", "--description", ""})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if capturedMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", capturedMethod)
	}
	if capturedPath != "/versions/7.json" {
		t.Errorf("path = %s, want /versions/7.json", capturedPath)
	}
	version, ok := capturedBody["version"].(map[string]any)
	if !ok {
		t.Fatalf("body missing version key: %+v", capturedBody)
	}
	if got, _ := version["name"].(string); got != "v1.2.1" {
		t.Errorf("name = %q, want v1.2.1", got)
	}
	if got, ok := version["description"].(string); !ok || got != "" {
		t.Errorf("description = %v, want empty string", version["description"])
	}
	if _, ok := version["status"]; ok {
		t.Errorf("status should be omitted when unchanged: %+v", version)
	}
}

// --- delete ---

func TestVersionDelete_Force(t *testing.T) {
	var capturedMethod, capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionDelete(f)
	cmd.SetArgs([]string{"7", "--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if capturedMethod != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", capturedMethod)
	}
	if capturedPath != "/versions/7.json" {
		t.Errorf("path = %s, want /versions/7.json", capturedPath)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "Deleted version 7") {
		t.Errorf("stderr = %q, want success message", stderr)
	}
}

func TestVersionDelete_Cancelled(t *testing.T) {
	f := testutil.NewFactory(t, "http://unused")
	f.IOStreams.In = strings.NewReader("n\n")

	cmd := newCmdVersionDelete(f)
	cmd.SetArgs([]string{"7"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "Delete cancelled") {
		t.Errorf("stderr = %q, want 'Delete cancelled'", stderr)
	}
}

// TestVersionDelete_ResolvesByName exercises the name-lookup branch of the
// shared resolveVersionID helper: project identifier lookup + versions list +
// name match + DELETE.
func TestVersionDelete_ResolvesByName(t *testing.T) {
	var deletePath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/projects/demo.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"project":{"id":1,"name":"Demo","identifier":"demo","description":"","status":1,"is_public":true,"created_on":"","updated_on":""}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/projects/demo/versions.json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"versions":[{"id":42,"project":{"id":1,"name":"Demo"},"name":"v1.2","status":"open","due_date":"","sharing":"none","description":"","created_on":"","updated_on":""}],"total_count":1}`))
		case r.Method == http.MethodDelete:
			deletePath = r.URL.Path
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected", http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionDelete(f)
	cmd.SetArgs([]string{"v1.2", "--project", "demo", "--force"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if deletePath != "/versions/42.json" {
		t.Errorf("delete path = %q, want /versions/42.json", deletePath)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "Deleted version 42") {
		t.Errorf("stderr = %q, want 'Deleted version 42'", stderr)
	}
}

// TestVersionDelete_NameWithoutProjectErrors pins the shared resolveVersionID
// error path: a non-numeric argument with no --project and no default project
// must fail fast before any HTTP traffic.
func TestVersionDelete_NameWithoutProjectErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected HTTP traffic: %s %s", r.Method, r.URL.Path)
		http.Error(w, "unexpected", http.StatusBadRequest)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionDelete(f)
	cmd.SetArgs([]string{"v1.2", "--force"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when resolving name without --project")
	}
	if !strings.Contains(err.Error(), "--project is required") {
		t.Errorf("error = %q, want contains '--project is required'", err.Error())
	}
}
