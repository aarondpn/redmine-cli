package role

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestRoleList_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"roles":[{"id":1,"name":"Manager","assignable":true},{"id":2,"name":"Reporter","assignable":false}]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdRoleList(f)
	cmd.SetArgs([]string{"--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	if !strings.Contains(stdout, `"name": "Manager"`) || !strings.Contains(stdout, `"assignable": true`) {
		t.Fatalf("unexpected JSON output:\n%s", stdout)
	}
}

func TestRoleList_Table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"roles":[{"id":1,"name":"Manager","assignable":true},{"id":2,"name":"Anonymous"}]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdRoleList(f)
	cmd.SetArgs([]string{"--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Manager", "Anonymous"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("table output missing %q:\n%s", want, stdout)
		}
	}
}

func TestRoleGet_DetailByName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/roles.json":
			_, _ = w.Write([]byte(`{"roles":[{"id":7,"name":"Reporter"}]}`))
		case "/roles/7.json":
			_, _ = w.Write([]byte(`{"role":{"id":7,"name":"Reporter","assignable":true,"issues_visibility":"default","permissions":["view_issues","add_issue_notes"]}}`))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdRoleGet(f)
	cmd.SetArgs([]string{"Reporter"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Reporter", "default", "view_issues"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("detail output missing %q:\n%s", want, stdout)
		}
	}
}

func TestRoleList_TableHasNoPhantomColumns(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"roles":[{"id":1,"name":"Manager"}]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdRoleList(f)
	cmd.SetArgs([]string{"--output", "table"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, bad := range []string{"Builtin", "Assignable"} {
		if strings.Contains(stdout, bad) {
			t.Errorf("table unexpectedly contains %q:\n%s", bad, stdout)
		}
	}
}
