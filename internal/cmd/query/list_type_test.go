package query

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestQueryList_Type(t *testing.T) {
	for _, tc := range []struct {
		args     []string
		wantType string
		hasType  bool
	}{
		{nil, "", false},
		{[]string{"--type", "issue"}, "", false},
		{[]string{"--type", "time_entry"}, "TimeEntryQuery", true},
	} {
		rec := testutil.NewQueryRecorder(t, `{"queries":[],"total_count":0}`)
		f := testutil.NewFactory(t, rec.URL)
		cmd := newCmdQueryList(f)
		cmd.SetArgs(append(tc.args, "--output", "json"))
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		q := rec.Last(t)
		if q.Has("type") != tc.hasType || q.Get("type") != tc.wantType {
			t.Errorf("queries list %v: type = %q (sent %v), want %q", tc.args, q.Get("type"), q.Has("type"), tc.wantType)
		}
	}
}

func TestQueryList_InvalidTypeRejectedBeforeHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request expected, got %s", r.URL)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdQueryList(f)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs([]string{"--type", "project"})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "use issue or time_entry") {
		t.Fatalf("err = %v", err)
	}
}
