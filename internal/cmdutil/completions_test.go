package cmdutil_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestCompleteRoles_FiltersBuiltInAndUnassignable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"roles":[{"id":1,"name":"Manager","assignable":true},{"id":2,"name":"Reporter","assignable":false},{"id":3,"name":"Anonymous","builtin":true},{"id":4,"name":"Non member","is_builtin":true}]}`))
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
