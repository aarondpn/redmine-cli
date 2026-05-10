package customfield

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

const listFixture = `{"custom_fields":[
	{"id":1,"name":"Severity","customized_type":"issue","field_format":"list","is_required":true,"is_filter":true,"searchable":false,"multiple":false,"possible_values":[{"value":"Low","label":"Low"},{"value":"High","label":"High"}],"trackers":[{"id":1,"name":"Bug"}]},
	{"id":2,"name":"Department","customized_type":"user","field_format":"string"}
]}`

func TestCustomFieldList_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(listFixture))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCustomFieldList(f)
	cmd.SetArgs([]string{"--output", "json"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{`"name": "Severity"`, `"customized_type": "issue"`, `"field_format": "list"`} {
		if !strings.Contains(stdout, want) {
			t.Errorf("JSON output missing %q:\n%s", want, stdout)
		}
	}
}

func TestCustomFieldList_Table(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(listFixture))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCustomFieldList(f)
	cmd.SetArgs([]string{"--output", "table"})

	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	stdout := testutil.Stdout(f)
	for _, want := range []string{"Severity", "Department", "issue", "user", "Format", "Required"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("table output missing %q:\n%s", want, stdout)
		}
	}
}
