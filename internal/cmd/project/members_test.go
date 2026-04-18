package project

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestCmdMembers_JSONEmptyDoesNotWarn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/demo/memberships.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"memberships":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMembers(f)
	cmd.SetArgs([]string{"demo", "--output", "json"})

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

func TestCmdMembers_JSONPaginatedDoesNotWarn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/demo/memberships.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("limit"); got != "1" {
			t.Fatalf("limit = %q, want 1", got)
		}
		if got := r.URL.Query().Get("offset"); got != "0" {
			t.Fatalf("offset = %q, want 0", got)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"memberships":[{"id":1,"project":{"id":1,"name":"Demo"},"user":{"id":2,"name":"Alice"},"roles":[{"id":3,"name":"Manager"}]}],"total_count":2}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMembers(f)
	cmd.SetArgs([]string{"demo", "--output", "json", "--limit", "1"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	stdout := testutil.Stdout(f)
	if !strings.Contains(stdout, `"name": "Alice"`) {
		t.Fatalf("stdout missing membership payload:\n%s", stdout)
	}
	if got := testutil.Stderr(f); got != "" {
		t.Fatalf("stderr = %q, want empty", got)
	}
}

func TestCmdMembers_CSVConfigDoesNotEmitANSI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/demo/memberships.json" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"memberships":[{"id":1,"project":{"id":1,"name":"Demo"},"user":{"id":2,"name":"Alice"},"roles":[{"id":3,"name":"Manager"}]}],"total_count":1}`))
	}))
	defer srv.Close()

	f := testutil.NewFactoryWithConfig(t, srv.URL, "output_format: csv\n")
	f.IOStreams.IsTTY = true
	cmd := newCmdMembers(f)
	cmd.SetArgs([]string{"demo"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	stdout := testutil.Stdout(f)
	if strings.Contains(stdout, "\x1b[") {
		t.Fatalf("csv output contains ANSI escapes:\n%q", stdout)
	}
	for _, want := range []string{"ID,User/Group,Roles", "1,Alice,Manager"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("csv output missing %q:\n%s", want, stdout)
		}
	}
}
