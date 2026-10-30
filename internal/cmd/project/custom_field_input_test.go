package project

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestCmdProjectUpdate_CustomFieldMultiValue(t *testing.T) {
	var posted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/projects/demo.json" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &posted)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUpdate(f)
	cmd.SetArgs([]string{"demo", "--custom-field", "5=a", "--custom-field", "5=b"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	proj := posted["project"].(map[string]any)
	want := map[string]any{"5": []any{"a", "b"}}
	if got := proj["custom_field_values"]; !reflect.DeepEqual(got, want) {
		t.Errorf("custom_field_values = %v, want %v", got, want)
	}
}

func TestCmdProjectCreate_IssueCustomFieldIgnoresOtherTypes(t *testing.T) {
	var posted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/custom_fields.json":
			_, _ = w.Write([]byte(`{"custom_fields":[
{"id":30,"name":"Severity","customized_type":"project","field_format":"string"},
{"id":21,"name":"Severity","customized_type":"issue","field_format":"string"}]}`))
		case "/projects.json":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &posted)
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"demo","name":"Demo"}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdCreate(f)
	cmd.SetArgs([]string{"--name", "Demo", "--identifier", "demo", "--issue-custom-field", "Severity", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	proj := posted["project"].(map[string]any)
	if got := proj["issue_custom_field_ids"]; !reflect.DeepEqual(got, []any{float64(21)}) {
		t.Errorf("issue_custom_field_ids = %v, want [21]", got)
	}
}
