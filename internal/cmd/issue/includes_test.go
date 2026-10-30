package issue

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestValidateIncludes(t *testing.T) {
	if err := validateGetIncludes([]string{"journals", "children", "relations", "attachments", "watchers", "changesets", "allowed_statuses", "reactions"}); err != nil {
		t.Errorf("get: unexpected error: %v", err)
	}
	if err := validateListIncludes([]string{"attachments", "relations"}); err != nil {
		t.Errorf("list: unexpected error: %v", err)
	}

	err := validateGetIncludes([]string{"bogus"})
	want := `unknown --include value "bogus" (allowed: allowed_statuses, attachments, changesets, children, journals, reactions, relations, watchers)`
	if err == nil || err.Error() != want {
		t.Errorf("get error = %v, want %q", err, want)
	}
	err = validateListIncludes([]string{"journals"})
	want = `unknown --include value "journals" (allowed: attachments, relations)`
	if err == nil || err.Error() != want {
		t.Errorf("list error = %v, want %q", err, want)
	}
}

func TestCompleteIncludes(t *testing.T) {
	got, _ := completeGetIncludes(nil, nil, "")
	want := []string{"allowed_statuses", "attachments", "changesets", "children", "journals", "reactions", "relations", "watchers"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("get completions = %v", got)
	}
	got, _ = completeListIncludes(nil, nil, "")
	if !reflect.DeepEqual(got, []string{"attachments", "relations"}) {
		t.Errorf("list completions = %v", got)
	}
}

func TestIncludeFlagsRejectUnknownValue(t *testing.T) {
	get := NewCmdGet(testutil.NewFactory(t, "http://127.0.0.1:1"))
	get.SetArgs([]string{"1", "--include", "bogus"})
	if err := get.Execute(); err == nil || !strings.Contains(err.Error(), `unknown --include value "bogus"`) {
		t.Errorf("get: err = %v", err)
	}

	list := NewCmdList(testutil.NewFactory(t, "http://127.0.0.1:1"))
	list.SetArgs([]string{"--include", "bogus"})
	if err := list.Execute(); err == nil || !strings.Contains(err.Error(), `unknown --include value "bogus"`) {
		t.Errorf("list: err = %v", err)
	}
}

func TestIssueGet_IncludeQuery(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"comma separated", []string{"--include", "journals,attachments"}, "journals,attachments"},
		{"spaced comma list", []string{"--include", "journals, attachments"}, "journals,attachments"},
		{"repeated", []string{"--include", "journals", "--include", "watchers"}, "journals,watchers"},
		{"shorthands merged", []string{"--include", "watchers", "--journals", "--relations"}, "watchers,journals,relations"},
		{"shorthand deduplicated", []string{"--include", "journals", "--journals"}, "journals"},
		{"attachments shorthand deduplicated", []string{"--include", "attachments", "--attachments"}, "attachments"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.Query().Get("include")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"issue":{"id":1,"subject":"S"}}`))
			}))
			defer srv.Close()

			cmd := NewCmdGet(testutil.NewFactory(t, srv.URL))
			cmd.SetArgs(append([]string{"1", "-o", "json"}, tt.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("include = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIssueList_IncludeQuery(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"comma separated", []string{"--include", "attachments,relations"}, "attachments,relations"},
		{"spaced comma list", []string{"--include", "attachments, relations"}, "attachments,relations"},
		{"repeated", []string{"--include", "attachments", "--include", "relations"}, "attachments,relations"},
		{"shorthands merged", []string{"--include", "relations", "--attachments"}, "relations,attachments"},
		{"shorthand deduplicated", []string{"--include", "attachments", "--attachments", "--relations"}, "attachments,relations"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.URL.Query().Get("include")
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"issues":[],"total_count":0}`))
			}))
			defer srv.Close()

			cmd := NewCmdList(testutil.NewFactory(t, srv.URL))
			cmd.SetArgs(append([]string{"-o", "json"}, tt.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("include = %q, want %q", got, tt.want)
			}
		})
	}
}

// An invalid --include must be reported before any network call, so it wins
// over authentication or resolver failures.
func TestIssueList_InvalidIncludeFailsBeforeNetwork(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	cmd := NewCmdList(testutil.NewFactory(t, srv.URL))
	cmd.SetArgs([]string{"--include", "bogus", "--project", "p", "--tracker", "Bug"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), `unknown --include value "bogus" (allowed: attachments, relations)`) {
		t.Errorf("err = %v", err)
	}
	if called {
		t.Error("server was contacted before include validation")
	}
}
