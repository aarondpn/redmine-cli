package issue

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestCmdIssueGet_JournalHeaderShowsID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"issue":{"id":1,"project":{"id":1,"name":"P"},"tracker":{"id":1,"name":"T"},"status":{"id":1,"name":"New"},"priority":{"id":1,"name":"Normal"},"author":{"id":1,"name":"A"},"subject":"s","journals":[` +
			`{"id":41,"user":{"id":1,"name":"Alice"},"notes":"first","created_on":"2026-01-01T00:00:00Z"},` +
			`{"id":42,"user":{"id":1,"name":"Alice"},"notes":"second","created_on":"2026-01-02T00:00:00Z","updated_by":{"id":1,"name":"Alice"}}]}}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdGet(f)
	cmd.SetArgs([]string{"1", "--journals", "--output", "table"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	got := testutil.Stdout(f)
	if !strings.Contains(got, "--- Alice (2026-01-01T00:00:00Z) [journal 41] ---") {
		t.Errorf("missing journal 41 header in %q", got)
	}
	if !strings.Contains(got, "--- Alice (2026-01-02T00:00:00Z) (edited) [journal 42] ---") {
		t.Errorf("missing edited journal 42 header in %q", got)
	}
}
