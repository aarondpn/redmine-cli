package wiki

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// newWikiGetServer serves the project lookup the resolver performs plus a
// single wiki page payload.
func newWikiGetServer(t *testing.T, pageJSON string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/projects/") && strings.Contains(r.URL.Path, "/wiki/") {
			_, _ = w.Write([]byte(pageJSON))
			return
		}
		_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"proj"}}`))
	}))
}

// TestGet_ProjectRow covers the project node Redmine 7.0 added to the wiki
// page API response (#43569): it is rendered when present and the row stays
// out of the detail view on older servers that omit it.
func TestGet_ProjectRow(t *testing.T) {
	t.Run("rendered when server sends project", func(t *testing.T) {
		srv := newWikiGetServer(t, `{"wiki_page":{"title":"MyPage","text":"body","version":2,"project":{"id":1,"name":"Apollo"},"created_on":"2026-01-01T00:00:00Z","updated_on":"2026-01-02T00:00:00Z"}}`)
		defer srv.Close()

		f := testutil.NewFactory(t, srv.URL)
		cmd := newCmdGet(f)
		cmd.SetArgs([]string{"MyPage", "--project", "proj", "--output", "table"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		stdout := testutil.Stdout(f)
		for _, want := range []string{"Project", "Apollo"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("detail output missing %q:\n%s", want, stdout)
			}
		}
	})

	t.Run("omitted on pre-7.0 response", func(t *testing.T) {
		srv := newWikiGetServer(t, `{"wiki_page":{"title":"MyPage","text":"body","version":2,"created_on":"2026-01-01T00:00:00Z","updated_on":"2026-01-02T00:00:00Z"}}`)
		defer srv.Close()

		f := testutil.NewFactory(t, srv.URL)
		cmd := newCmdGet(f)
		cmd.SetArgs([]string{"MyPage", "--project", "proj", "--output", "table"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		// Match the row label, not a bare "Project" substring, so unrelated
		// future rows or page text containing the word cannot fail this.
		if stdout := testutil.Stdout(f); strings.Contains(stdout, "Project:") {
			t.Errorf("detail output unexpectedly contains a Project row:\n%s", stdout)
		}
	})
}
