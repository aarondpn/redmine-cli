package project

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestCmdProjectList_StatusSortAndFilter(t *testing.T) {
	rec := testutil.NewQueryRecorder(t, `{"projects":[],"total_count":0}`)
	f := testutil.NewFactory(t, rec.URL)
	cmd := newCmdList(f)
	cmd.SetArgs([]string{
		"--status", "all",
		"--sort", "name:desc",
		"--filter", "name=~web",
		"--output", "json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	q := rec.Last(t)
	testutil.WantQueryValues(t, q, "status", "1|5|9")
	testutil.WantQueryValues(t, q, "sort", "name:desc")
	testutil.WantQueryValues(t, q, "name", "~web")
}

func TestCmdProjectList_StatusKeywords(t *testing.T) {
	for in, want := range map[string]string{"active": "1", "closed": "5", "5": "5"} {
		rec := testutil.NewQueryRecorder(t, `{"projects":[],"total_count":0}`)
		f := testutil.NewFactory(t, rec.URL)
		cmd := newCmdList(f)
		cmd.SetArgs([]string{"--status", in, "--output", "json"})
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		testutil.WantQueryValues(t, rec.Last(t), "status", want)
	}
}

func TestCmdProjectList_NoStatusByDefault(t *testing.T) {
	rec := testutil.NewQueryRecorder(t, `{"projects":[],"total_count":0}`)
	f := testutil.NewFactory(t, rec.URL)
	cmd := newCmdList(f)
	cmd.SetArgs([]string{"--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if q := rec.Last(t); q.Has("status") || q.Has("sort") {
		t.Errorf("unexpected status/sort: %s", q.Encode())
	}
}

// A --filter status= replaces the value --status sends.
func TestCmdProjectList_FilterOverridesStatus(t *testing.T) {
	rec := testutil.NewQueryRecorder(t, `{"projects":[],"total_count":0}`)
	f := testutil.NewFactory(t, rec.URL)
	cmd := newCmdList(f)
	cmd.SetArgs([]string{"--status", "active", "--filter", "status=5", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	testutil.WantQueryValues(t, rec.Last(t), "status", "5")
}

func TestCmdProjectList_InvalidStatusRejectedBeforeHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request expected, got %s", r.URL)
	}))
	defer srv.Close()
	for _, status := range []string{"archived", "bogus"} {
		f := testutil.NewFactory(t, srv.URL)
		cmd := newCmdList(f)
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		cmd.SetArgs([]string{"--status", status})
		err := cmd.Execute()
		if err == nil || !strings.Contains(err.Error(), status) {
			t.Errorf("--status %s: err = %v", status, err)
		}
	}
}

func TestProjectStatusLabel(t *testing.T) {
	for status, want := range map[int]string{1: "active", 5: "closed", 9: "archived", 10: "10"} {
		if got := projectStatusLabel(status); got != want {
			t.Errorf("projectStatusLabel(%d) = %q, want %q", status, got, want)
		}
	}
}
