package category

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestCategoryList_EmptyJSON(t *testing.T) {
	srv := httptest.NewServer(withProject(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issue_categories":[],"total_count":0}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryList(f)
	cmd.SetArgs([]string{"--project", "1", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if got := testutil.Stdout(f); got != "[]\n" {
		t.Fatalf("stdout = %q, want %q", got, "[]\n")
	}
}

func TestCategoryList_Table(t *testing.T) {
	srv := httptest.NewServer(withProject(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issue_categories":[{"id":1,"name":"UI","assigned_to":{"id":2,"name":"Alice"}}],"total_count":1}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCategoryList(f)
	cmd.SetArgs([]string{"--project", "1", "--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"UI", "Alice"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("table output missing %q:\n%s", want, stdout)
		}
	}
}

// withProject answers the resolver's /projects/1.json lookup with a real
// project so the command receives a non-empty identifier, and delegates every
// other request to h.
func withProject(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/projects/1.json" {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"project":{"id":1,"name":"Demo","identifier":"demo"}}`))
			return
		}
		h(w, r)
	}
}
