package tracker

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestTrackerList_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"trackers":[{"id":1,"name":"Bug","description":"Bug reports"}]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTrackerList(f)
	cmd.SetArgs([]string{"--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	if !strings.Contains(stdout, `"name": "Bug"`) {
		t.Errorf("JSON output missing tracker name:\n%s", stdout)
	}
}

func TestTrackerList_Table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"trackers":[{"id":1,"name":"Bug","description":"Bug reports"},{"id":2,"name":"Feature","description":""}]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTrackerList(f)
	cmd.SetArgs([]string{"--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Bug", "Feature"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("table output missing %q:\n%s", want, stdout)
		}
	}
}
