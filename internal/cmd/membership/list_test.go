package membership

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

const membershipsBody = `{"memberships":[
  {"id":1,"project":{"id":1,"name":"Demo"},"user":{"id":5,"name":"Ann Dev"},"roles":[{"id":3,"name":"Manager"},{"id":4,"name":"Developer","inherited":true}]},
  {"id":2,"project":{"id":1,"name":"Demo"},"group_anonymous":{"id":13,"name":"Anonymous users"},"roles":[{"id":6,"name":"Anonymous"}]},
  {"id":3,"project":{"id":1,"name":"Demo"},"group_non_member":{"id":12,"name":"Non member users"},"roles":[{"id":5,"name":"Non member"}]}
],"total_count":3}`

// membershipsServer answers the project lookup the command performs before
// listing, then the memberships index.
func membershipsServer() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/projects/demo.json" {
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"demo","name":"Demo"}}`))
			return
		}
		_, _ = w.Write([]byte(membershipsBody))
	})
}

func TestMembershipList_BuiltinGroupsAreNamed(t *testing.T) {
	srv := httptest.NewServer(membershipsServer())
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMembershipList(f)
	cmd.SetArgs([]string{"--project", "demo"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := testutil.Stdout(f)
	for _, want := range []string{"Ann Dev", "Anonymous users (group)", "Non member users (group)", "Manager, Developer"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "unknown") {
		t.Errorf("table shows an unnamed principal:\n%s", out)
	}
}

func TestMembershipList_JSONKeepsInheritedAndBuiltinKeys(t *testing.T) {
	srv := httptest.NewServer(membershipsServer())
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMembershipList(f)
	cmd.SetArgs([]string{"--project", "demo", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	out := testutil.Stdout(f)
	for _, want := range []string{`"inherited": true`, `"group_anonymous"`, `"group_non_member"`} {
		if !strings.Contains(out, want) {
			t.Errorf("json missing %s:\n%s", want, out)
		}
	}
	if strings.Count(out, `"inherited"`) != 1 {
		t.Errorf("inherited must only appear on the inherited role:\n%s", out)
	}
}
