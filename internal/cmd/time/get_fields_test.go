package time

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func runTimeGet(t *testing.T, body string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeGet(f)
	cmd.SetArgs([]string{"42"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	return testutil.Stdout(f)
}

func TestTimeGet_CustomFields(t *testing.T) {
	got := runTimeGet(t, `{"time_entry":{"id":42,"hours":1,"custom_fields":[{"id":1,"name":"Billable","value":"yes"},{"id":2,"name":"Blank","value":""}]}}`)
	if !strings.Contains(got, "Custom Fields:") || !strings.Contains(got, "Billable: yes") || strings.Contains(got, "Blank") {
		t.Errorf("unexpected output:\n%s", got)
	}
	if got := runTimeGet(t, `{"time_entry":{"id":42,"hours":1}}`); strings.Contains(got, "Custom Fields") {
		t.Errorf("custom fields row shown for minimal payload:\n%s", got)
	}
}
