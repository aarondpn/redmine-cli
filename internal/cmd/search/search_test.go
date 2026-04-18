package search

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestCmdSearchProjects_EmptyJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdSearchProjects(f)
	cmd.SetArgs([]string{"nonexistent", "--output", "json"})

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

func TestCmdSearchProjects_EmptyTableWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdSearchProjects(f)
	cmd.SetArgs([]string{"nonexistent", "--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	stderr := testutil.Stderr(f)
	if stderr == "" {
		t.Fatal("stderr is empty, want warning about no projects found")
	}
}
