package query

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestQueryList_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"queries":[{"id":1,"name":"My open","is_public":false},{"id":2,"name":"All bugs","is_public":true,"project_id":7}],"total_count":2}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdQueryList(f)
	cmd.SetArgs([]string{"--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	if !strings.Contains(stdout, `"name": "My open"`) || !strings.Contains(stdout, `"name": "All bugs"`) {
		t.Fatalf("unexpected JSON output:\n%s", stdout)
	}
}

func TestQueryList_Table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"queries":[{"id":1,"name":"Mine","is_public":false},{"id":2,"name":"Team","is_public":true,"project_id":4}],"total_count":2}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdQueryList(f)
	cmd.SetArgs([]string{"--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Mine", "Team", "private", "public", "global", "project 4"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("table output missing %q:\n%s", want, stdout)
		}
	}
}

func TestQueryList_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"queries":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdQueryList(f)
	cmd.SetArgs([]string{"--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if stderr := testutil.Stderr(f); !strings.Contains(stderr, "No queries found") {
		t.Fatalf("stderr = %q, want empty-state warning", stderr)
	}
}

func TestQueryGet_ByNumericID(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/queries.json" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"queries":[{"id":3,"name":"Other"},{"id":7,"name":"Sprint","is_public":true,"project_id":3}],"total_count":2}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdQueryGet(f)
	cmd.SetArgs([]string{"7"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("expected exactly one /queries.json call, got %d", calls)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Sprint", "public", "project 3"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("detail output missing %q:\n%s", want, stdout)
		}
	}
}

func TestQueryGet_ByNumericID_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"queries":[{"id":1,"name":"Other"}],"total_count":1}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdQueryGet(f)
	cmd.SetArgs([]string{"99"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected not-found error")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("error = %v, want not-found message", err)
	}
}

func TestQueryGet_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"queries":[{"id":7,"name":"Sprint","is_public":true,"project_id":3}],"total_count":1}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdQueryGet(f)
	cmd.SetArgs([]string{"Sprint", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{`"id": 7`, `"name": "Sprint"`, `"is_public": true`} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("JSON output missing %q:\n%s", want, stdout)
		}
	}
}

func TestQueryGet_DetailByName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/queries.json" {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"queries":[{"id":7,"name":"Sprint","is_public":true,"project_id":3}],"total_count":1}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdQueryGet(f)
	cmd.SetArgs([]string{"Sprint"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Sprint", "public", "project 3"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("detail output missing %q:\n%s", want, stdout)
		}
	}
}
