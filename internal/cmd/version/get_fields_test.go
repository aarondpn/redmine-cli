package version

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func runVersionGet(t *testing.T, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionGet(f)
	cmd.SetArgs([]string{"5"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	return testutil.Stdout(f)
}

func TestVersionGet_ExtraFields(t *testing.T) {
	got := runVersionGet(t, `{"version":{"id":5,"name":"v1","status":"open","wiki_page_title":"Release_1","estimated_hours":10,"spent_hours":2.5,"custom_fields":[{"id":1,"name":"Owner","value":"Bob"}]}}`)
	for _, want := range []string{"Wiki Page:", "Release_1", "Estimated Hours:", "10.00", "Spent Hours:", "2.50", "Owner: Bob"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	got = runVersionGet(t, `{"version":{"id":5,"name":"v1","status":"open"}}`)
	for _, unwanted := range []string{"Wiki", "Hours", "Custom"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("unexpected %q in:\n%s", unwanted, got)
		}
	}
}
