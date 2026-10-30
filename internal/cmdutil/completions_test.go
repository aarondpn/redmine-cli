package cmdutil_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestCompleteRoles_FiltersUnassignable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"roles":[{"id":1,"name":"Manager","assignable":true},{"id":2,"name":"Reporter","assignable":false}]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	got, directive := cmdutil.CompleteRoles(f)(&cobra.Command{}, nil, "")
	if directive != cobra.ShellCompDirectiveNoFileComp {
		t.Fatalf("directive = %v, want %v", directive, cobra.ShellCompDirectiveNoFileComp)
	}
	if len(got) != 1 || got[0] != "Manager\tID 1" {
		t.Fatalf("completions = %v, want [Manager\\tID 1]", got)
	}
}

func TestCompleteAssignees_ProjectMembersIncludeGroups(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/projects/demo/memberships.json" {
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"memberships":[{"id":1,"project":{"id":7,"name":"Demo"},"user":{"id":3,"name":"Alice Smith"},"roles":[]},{"id":2,"project":{"id":7,"name":"Demo"},"group":{"id":12,"name":"Backend Team"},"roles":[]}],"total_count":2}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := &cobra.Command{}
	cmd.Flags().String("project", "", "")
	_ = cmd.Flags().Set("project", "demo")

	got, _ := cmdutil.CompleteAssignees(f)(cmd, nil, "")
	want := []string{"me\tCurrent user", "none\tNo assignee", "Alice Smith\tuser, ID 3", "Backend Team\tgroup, ID 12"}
	if len(got) != len(want) {
		t.Fatalf("completions = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("completions = %q, want %q", got, want)
		}
	}
}

// Commands without --project (issues update / assign) use the project of
// the issue in the first positional argument.
func TestCompleteAssignees_UsesIssueProject(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/issues/5.json":
			_, _ = w.Write([]byte(`{"issue":{"id":5,"project":{"id":7,"name":"Demo"}}}`))
		case "/projects/7/memberships.json":
			_, _ = w.Write([]byte(`{"memberships":[{"id":2,"project":{"id":7,"name":"Demo"},"group":{"id":12,"name":"Backend Team"},"roles":[]}],"total_count":1}`))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	got, _ := cmdutil.CompleteAssignees(f)(&cobra.Command{}, []string{"5"}, "Back")
	if len(got) != 1 || got[0] != "Backend Team\tgroup, ID 12" {
		t.Fatalf("completions = %q, want [Backend Team]", got)
	}
}

func TestCompleteAssignees_WithoutProjectOffersNoneAndMe(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	got, _ := cmdutil.CompleteAssignees(f)(&cobra.Command{}, nil, "")
	if len(got) != 2 || got[0] != "none\tNo assignee" || got[1] != "me\tCurrent user" {
		t.Fatalf("completions = %q, want [none me]", got)
	}
}

// Candidates come from the server, so they are neutralized before cobra
// prints them for the shell: one with a line break would turn into several
// candidates and is dropped, control characters in values and descriptions
// are shown in caret notation, and line breaks or tabs in a description
// become spaces.
func TestCompletions_SanitizeServerData(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"versions":[` +
			`{"id":1,"name":"1.0","status":"open"},` +
			`{"id":2,"name":"evil\nrm -rf ~","status":"open"},` +
			`{"id":3,"name":"evil\r","status":"open"},` +
			`{"id":4,"name":"2.0\u001b]52;c;ZXZpbA==\u0007","status":"open\u001b[2J"},` +
			`{"id":5,"name":"3.0","status":"open\n:0\nfake\tline"}` +
			`],"total_count":5}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := &cobra.Command{}
	cmd.Flags().String("project", "", "")
	_ = cmd.Flags().Set("project", "demo")

	got, _ := cmdutil.CompleteVersions(f)(cmd, nil, "")
	want := []string{
		"1.0\topen",
		"2.0^[]52;c;ZXZpbA==^G\topen^[[2J",
		"3.0\topen :0 fake line",
	}
	if len(got) != len(want) {
		t.Fatalf("completions = %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("completions = %q, want %q", got, want)
		}
	}
}
