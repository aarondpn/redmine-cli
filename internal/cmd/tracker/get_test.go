package tracker

import (
	"bytes"
	"encoding/csv"
	"net/http"
	"net/http/httptest"
	"slices"
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

func TestTrackerGet_CSV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"trackers":[{"id":1,"name":"Bug","description":"Bug reports","default_status":{"id":2,"name":"New"},"enabled_standard_fields":["description","due_date"]}]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTrackerGet(f)
	cmd.SetArgs([]string{"1", "--output", "csv"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}

	rows, err := csv.NewReader(bytes.NewBufferString(testutil.Stdout(f))).ReadAll()
	if err != nil {
		t.Fatalf("decode CSV: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("CSV rows = %d, want 2", len(rows))
	}

	wantHeaders := []string{"ID", "Name", "Default Status", "Description", "Enabled Standard Fields"}
	if !slices.Equal(rows[0], wantHeaders) {
		t.Fatalf("CSV header mismatch:\n got: %v\nwant: %v", rows[0], wantHeaders)
	}

	wantRow := []string{"1", "Bug", "New (ID: 2)", "Bug reports", "description, due_date"}
	if !slices.Equal(rows[1], wantRow) {
		t.Fatalf("CSV row mismatch:\n got: %v\nwant: %v", rows[1], wantRow)
	}
}
