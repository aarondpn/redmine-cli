package customfield

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

const detailFixture = `{"custom_fields":[
	{"id":1,"name":"Severity","customized_type":"issue","field_format":"list","is_required":true,"is_filter":true,"searchable":true,"multiple":false,"default_value":"Low","visible":true,"possible_values":[{"value":"Low","label":"Low"},{"value":"High","label":"High"}],"trackers":[{"id":1,"name":"Bug"}],"roles":[{"id":3,"name":"Manager"}]},
	{"id":2,"name":"Department","customized_type":"user","field_format":"string"}
]}`

func TestCustomFieldGet_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(detailFixture))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCustomFieldGet(f)
	cmd.SetArgs([]string{"Severity", "--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{`"name": "Severity"`, `"possible_values": [`, `"trackers": [`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("JSON output missing %q:\n%s", want, stdout)
		}
	}
}

func TestCustomFieldGet_TableByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(detailFixture))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCustomFieldGet(f)
	cmd.SetArgs([]string{"1", "--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Severity", "Format", "list", "Possible Values", "Low, High", "Bug (ID: 1)", "Manager (ID: 3)"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("detail output missing %q:\n%s", want, stdout)
		}
	}
}

func TestCustomFieldGet_CSV(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(detailFixture))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCustomFieldGet(f)
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

	wantHeaders := []string{"ID", "Name", "Type", "Format", "Required", "Filter", "Searchable", "Multiple", "Default", "Possible Values", "Trackers", "Roles"}
	if !slices.Equal(rows[0], wantHeaders) {
		t.Fatalf("CSV header mismatch:\n got: %v\nwant: %v", rows[0], wantHeaders)
	}

	wantRow := []string{"1", "Severity", "issue", "list", "yes", "yes", "yes", "no", "Low", "Low, High", "Bug (ID: 1)", "Manager (ID: 3)"}
	if !slices.Equal(rows[1], wantRow) {
		t.Fatalf("CSV row mismatch:\n got: %v\nwant: %v", rows[1], wantRow)
	}
}

func TestCustomFieldGet_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"custom_fields":[{"id":1,"name":"Severity"}]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCustomFieldGet(f)
	cmd.SetArgs([]string{"99"})

	err := cmd.Execute()
	if err == nil {
		t.Fatal("Execute returned nil, want not-found error")
	}
}
