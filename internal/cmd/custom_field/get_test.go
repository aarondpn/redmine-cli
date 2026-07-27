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

// redmine7Fixture carries the fields Redmine 7.0 added to /custom_fields.json:
// is_for_all plus the associated projects (#44153) and roles on a non-issue
// custom field (#44152).
const redmine7Fixture = `{"custom_fields":[
	{"id":1,"name":"Billing Code","description":"Cost centre","customized_type":"time_entry","field_format":"string","visible":true,"editable":true,"is_for_all":false,"projects":[{"id":7,"name":"Apollo"}],"roles":[{"id":3,"name":"Manager"}]}
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

	wantHeaders := []string{"ID", "Name", "Type", "Format", "Required", "Filter", "Searchable", "Multiple", "Default", "Possible Values", "Trackers", "Roles", "For All", "Projects"}
	if !slices.Equal(rows[0], wantHeaders) {
		t.Fatalf("CSV header mismatch:\n got: %v\nwant: %v", rows[0], wantHeaders)
	}

	wantRow := []string{"1", "Severity", "issue", "list", "yes", "yes", "yes", "no", "Low", "Low, High", "Bug (ID: 1)", "Manager (ID: 3)", "", ""}
	if !slices.Equal(rows[1], wantRow) {
		t.Fatalf("CSV row mismatch:\n got: %v\nwant: %v", rows[1], wantRow)
	}
}

// TestCustomFieldGet_Redmine7Fields covers the Redmine 7.0 additions to
// /custom_fields.json and the tri-state handling of is_for_all, which older
// servers omit entirely.
func TestCustomFieldGet_Redmine7Fields(t *testing.T) {
	t.Run("table surfaces scope and roles", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(redmine7Fixture))
		}))
		defer srv.Close()

		f := testutil.NewFactory(t, srv.URL)
		cmd := newCmdCustomFieldGet(f)
		cmd.SetArgs([]string{"1", "--output", "table"})

		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		stdout := testutil.Stdout(f)
		for _, want := range []string{"For All Projects", "Editable", "Description", "Cost centre", "Projects", "Apollo (ID: 7)", "Manager (ID: 3)"} {
			if !strings.Contains(stdout, want) {
				t.Errorf("detail output missing %q:\n%s", want, stdout)
			}
		}
	})

	t.Run("csv carries scope columns", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(redmine7Fixture))
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
		if got := rows[1][len(rows[1])-2]; got != "no" {
			t.Errorf("For All column = %q, want %q", got, "no")
		}
		if got := rows[1][len(rows[1])-1]; got != "Apollo (ID: 7)" {
			t.Errorf("Projects column = %q, want %q", got, "Apollo (ID: 7)")
		}
	})

	t.Run("flag rows omitted when the server does not send them", func(t *testing.T) {
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
		for _, unwanted := range []string{"For All Projects", "Editable"} {
			if strings.Contains(stdout, unwanted) {
				t.Errorf("detail output unexpectedly contains %q when the flag is absent:\n%s", unwanted, stdout)
			}
		}
	})
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
