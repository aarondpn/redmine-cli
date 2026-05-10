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

// TestCustomFieldGet_TableOptionalRows verifies that the detail renderer
// surfaces Default/Regexp/MinLength/MaxLength when the API provides them and
// omits the rows entirely when the values are absent. The two cases share a
// fixture pair so a regression where one branch swallows the other shows up.
func TestCustomFieldGet_TableOptionalRows(t *testing.T) {
	t.Run("rows present when fields set", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"custom_fields":[
				{"id":1,"name":"Ticket","customized_type":"issue","field_format":"string","default_value":"none","regexp":"^[A-Z]+$","min_length":3,"max_length":20}
			]}`))
		}))
		defer srv.Close()

		f := testutil.NewFactory(t, srv.URL)
		cmd := newCmdCustomFieldGet(f)
		cmd.SetArgs([]string{"1", "--output", "table"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		stdout := testutil.Stdout(f)
		for _, want := range []string{"Default", "none", "Regexp", "^[A-Z]+$", "Min Length", "3", "Max Length", "20"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("detail output missing %q:\n%s", want, stdout)
			}
		}
	})

	t.Run("rows omitted when fields absent", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"custom_fields":[
				{"id":1,"name":"Plain","customized_type":"issue","field_format":"string"}
			]}`))
		}))
		defer srv.Close()

		f := testutil.NewFactory(t, srv.URL)
		cmd := newCmdCustomFieldGet(f)
		cmd.SetArgs([]string{"1", "--output", "table"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		stdout := testutil.Stdout(f)
		for _, unwanted := range []string{"Default", "Regexp", "Min Length", "Max Length", "Possible Values", "Trackers", "Roles"} {
			if strings.Contains(stdout, unwanted) {
				t.Errorf("detail output unexpectedly contains %q for empty field:\n%s", unwanted, stdout)
			}
		}
	})
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
