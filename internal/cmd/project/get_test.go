package project

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestCmdProjectGet_IncludePropagatesAndRenders(t *testing.T) {
	var gotInclude string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInclude = r.URL.Query().Get("include")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "project": {
                "id": 1,
                "name": "Demo",
                "identifier": "demo",
                "description": "",
                "status": 1,
                "is_public": true,
                "homepage": "https://example.test",
                "trackers": [{"id": 1, "name": "Bug"}, {"id": 2, "name": "Feature"}],
                "enabled_modules": [{"id": 10, "name": "issue_tracking"}, {"id": 11, "name": "wiki"}],
                "issue_categories": [{"id": 5, "name": "Backend"}],
                "time_entry_activities": [{"id": 8, "name": "Development", "is_default": true, "active": true}],
                "issue_custom_fields": [{"id": 3, "name": "Severity"}]
            }
        }`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGet(f)
	cmd.SetArgs([]string{"demo", "--include", "trackers,enabled_modules,issue_categories,time_entry_activities,issue_custom_fields", "--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	want := "trackers,enabled_modules,issue_categories,time_entry_activities,issue_custom_fields"
	if gotInclude != want {
		t.Errorf("include = %q, want %q", gotInclude, want)
	}

	stdout := testutil.Stdout(f)
	for _, fragment := range []string{
		"Homepage",
		"https://example.test",
		"Trackers",
		"Bug (#1)",
		"Feature (#2)",
		"Enabled Modules",
		"issue_tracking (#10)",
		"Issue Categories",
		"Backend (#5)",
		"Time Entry Activities",
		"Development (#8) [default]",
		"Issue Custom Fields",
		"Severity (#3)",
	} {
		if !strings.Contains(stdout, fragment) {
			t.Errorf("stdout missing %q\n--- stdout ---\n%s", fragment, stdout)
		}
	}
}

func TestCmdProjectGet_InvalidInclude(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("server should not be called for invalid include; got %s", r.URL.String())
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGet(f)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"demo", "--include", "bogus"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unknown include value")
	}
	if !strings.Contains(err.Error(), "bogus") {
		t.Errorf("err = %v, want mention of bogus", err)
	}
}

func TestCmdProjectGet_JSONPreservesExtendedFields(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
            "project": {
                "id": 1,
                "name": "Demo",
                "identifier": "demo",
                "homepage": "https://example.test",
                "default_assigned_to": {"id": 7, "name": "Bob"},
                "default_version": {"id": 4, "name": "v1.0"}
            }
        }`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdGet(f)
	cmd.SetArgs([]string{"demo", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	var got struct {
		Homepage          string `json:"homepage"`
		DefaultAssignedTo struct {
			Name string `json:"name"`
		} `json:"default_assigned_to"`
		DefaultVersion struct {
			Name string `json:"name"`
		} `json:"default_version"`
	}
	if err := json.Unmarshal([]byte(testutil.Stdout(f)), &got); err != nil {
		t.Fatalf("decode stdout: %v\n%s", err, testutil.Stdout(f))
	}
	if got.Homepage != "https://example.test" {
		t.Errorf("homepage = %q", got.Homepage)
	}
	if got.DefaultAssignedTo.Name != "Bob" {
		t.Errorf("default_assigned_to.name = %q", got.DefaultAssignedTo.Name)
	}
	if got.DefaultVersion.Name != "v1.0" {
		t.Errorf("default_version.name = %q", got.DefaultVersion.Name)
	}
}
