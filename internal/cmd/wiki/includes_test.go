package wiki

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestValidateGetIncludes(t *testing.T) {
	if err := validateGetIncludes([]string{"attachments"}); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	err := validateGetIncludes([]string{"bogus"})
	want := `unknown --include value "bogus" (allowed: attachments)`
	if err == nil || err.Error() != want {
		t.Errorf("error = %v, want %q", err, want)
	}
}

func TestCompleteGetIncludes(t *testing.T) {
	got, _ := completeGetIncludes(nil, nil, "")
	if !reflect.DeepEqual(got, []string{"attachments"}) {
		t.Errorf("completions = %v", got)
	}
}

func TestWikiGetRejectsUnknownInclude(t *testing.T) {
	cmd := newCmdGet(nil)
	cmd.SetArgs([]string{"Home", "--include", "bogus"})
	if err := cmd.Execute(); err == nil || !strings.Contains(err.Error(), `unknown --include value "bogus"`) {
		t.Errorf("err = %v", err)
	}
}

func TestWikiGet_IncludeQuery(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"single", []string{"--include", "attachments"}},
		{"spaced", []string{"--include", " attachments "}},
		{"repeated duplicate", []string{"--include", "attachments", "--include", "attachments"}},
		{"comma duplicate", []string{"--include", "attachments,attachments"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if strings.Contains(r.URL.Path, "/wiki/") {
					got = r.URL.Query().Get("include")
					_, _ = w.Write([]byte(`{"wiki_page":{"title":"Home","version":1}}`))
					return
				}
				_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"proj"}}`))
			}))
			defer srv.Close()

			cmd := newCmdGet(testutil.NewFactory(t, srv.URL))
			cmd.SetArgs(append([]string{"Home", "--project", "proj", "-o", "json"}, tt.args...))
			if err := cmd.Execute(); err != nil {
				t.Fatal(err)
			}
			if got != "attachments" {
				t.Errorf("include = %q, want attachments", got)
			}
		})
	}
}
