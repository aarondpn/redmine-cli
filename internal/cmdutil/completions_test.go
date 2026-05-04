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
