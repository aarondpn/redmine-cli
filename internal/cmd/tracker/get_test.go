package tracker

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestTrackerGet_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"trackers":[{"id":1,"name":"Bug","description":"Bug reports","default_status":{"id":2,"name":"New"},"enabled_standard_fields":["description","due_date"]}]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTrackerGet(f)
	cmd.SetArgs([]string{"Bug", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{`"name": "Bug"`, `"default_status": {`, `"enabled_standard_fields": [`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("JSON output missing %q:\n%s", want, stdout)
		}
	}
}

func TestTrackerGet_Table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"trackers":[{"id":1,"name":"Bug","description":"Bug reports","default_status":{"id":2,"name":"New"},"enabled_standard_fields":["description","due_date"]}]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTrackerGet(f)
	cmd.SetArgs([]string{"1", "--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Default Status", "New (ID: 2)", "Enabled Standard Fields", "description, due_date"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("detail output missing %q:\n%s", want, stdout)
		}
	}
}
