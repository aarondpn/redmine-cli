package group

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestValidateGetIncludes(t *testing.T) {
	if err := validateGetIncludes([]string{"users", "memberships"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err := validateGetIncludes([]string{"bogus"})
	want := `unknown --include value "bogus" (allowed: memberships, users)`
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestCompleteGetIncludes(t *testing.T) {
	got, _ := completeGetIncludes(nil, nil, "")
	if !reflect.DeepEqual(got, []string{"memberships", "users"}) {
		t.Errorf("completions = %v", got)
	}
}

func TestGroupGetRejectsUnknownInclude(t *testing.T) {
	cmd := newCmdGroupGet(nil)
	cmd.SetArgs([]string{"1", "--include", "bogus"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), `unknown --include value "bogus"`) {
		t.Errorf("err = %v", err)
	}
}

func TestGroupGet_IncludeFlagVariants(t *testing.T) {
	const body = `{"group":{"id":1,"name":"Developers","users":[{"id":5,"name":"Ann"}],"memberships":[{"id":7,"project":{"id":3,"name":"Apollo"},"roles":[]}]}}`
	tests := []struct {
		name     string
		args     []string
		wantWarn []string
	}{
		{"spaced comma list", []string{"--include", "users, memberships"}, nil},
		{"include flag", []string{"--include", "users,memberships"}, nil},
		{"repeated include flag", []string{"--include", "users", "--include", "memberships"}, nil},
		// DEPRECATED(remove-after: 2027-01-31): --include-users/--include-memberships alias rows; delete them
		{"deprecated aliases", []string{"--include-users", "--include-memberships"}, []string{"--include-users", "--include-memberships", "2027-01-31"}},
		// DEPRECATED(remove-after: 2027-01-31): --include-users alias row; delete it
		{"aliases combined with include", []string{"--include", "users,memberships", "--include-users", "--include-memberships"}, []string{"--include-users"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.URL.Query().Get("include"); got != "users,memberships" {
					t.Errorf("include = %q, want users,memberships", got)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()

			f := testutil.NewFactory(t, srv.URL)
			cmd := newCmdGroupGet(f)
			var stderr bytes.Buffer
			cmd.SetOut(&stderr)
			cmd.SetArgs(append([]string{"1", "-o", "json"}, tt.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}

			stdout := testutil.Stdout(f)
			if strings.Contains(stdout, "deprecated") {
				t.Errorf("deprecation warning leaked to stdout:\n%s", stdout)
			}
			if !strings.Contains(stdout, `"Ann"`) || !strings.Contains(stdout, `"Apollo"`) {
				t.Errorf("stdout missing included data:\n%s", stdout)
			}
			for _, want := range tt.wantWarn {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr %q missing %q", stderr.String(), want)
				}
			}
			if tt.wantWarn == nil && stderr.Len() > 0 {
				t.Errorf("unexpected stderr output: %q", stderr.String())
			}
		})
	}
}

// DEPRECATED(remove-after: 2027-01-31): tests the --include-users/--include-memberships aliases; delete this test
func TestGroupGet_DeprecatedAliasesHidden(t *testing.T) {
	cmd := newCmdGroupGet(nil)
	for _, name := range []string{"include-users", "include-memberships"} {
		fl := cmd.Flags().Lookup(name)
		if fl == nil || !fl.Hidden || fl.Deprecated == "" {
			t.Errorf("--%s should be a hidden deprecated flag, got %+v", name, fl)
		}
	}
}
