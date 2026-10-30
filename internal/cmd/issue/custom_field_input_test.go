package issue

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func createdIssueCFVs(t *testing.T, posted map[string]any) map[string]any {
	t.Helper()
	issue, ok := posted["issue"].(map[string]any)
	if !ok {
		t.Fatalf("posted body missing issue wrapper: %v", posted)
	}
	cfvs, ok := issue["custom_field_values"].(map[string]any)
	if !ok {
		t.Fatalf("posted body missing custom_field_values: %v", issue)
	}
	return cfvs
}

func TestCmdIssueCreate_CustomFieldMultiValue(t *testing.T) {
	var posted map[string]any
	srv := newCreateIssueServer(t, &posted)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdCreate(f)
	cmd.SetArgs([]string{
		"--project", "17", "--subject", "test",
		"--custom-field", "5=a",
		"--custom-field", "5=b c",
		"--custom-field", "6=x",
		"--output", "json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	want := map[string]any{"5": []any{"a", "b c"}, "6": "x"}
	if got := createdIssueCFVs(t, posted); !reflect.DeepEqual(got, want) {
		t.Errorf("custom_field_values = %v, want %v", got, want)
	}
}

func TestCmdIssueCreate_CustomFieldClear(t *testing.T) {
	var posted map[string]any
	srv := newCreateIssueServer(t, &posted)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdCreate(f)
	cmd.SetArgs([]string{"--project", "17", "--subject", "test", "--custom-field", "5=", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if got := createdIssueCFVs(t, posted); !reflect.DeepEqual(got, map[string]any{"5": ""}) {
		t.Errorf("custom_field_values = %v", got)
	}
}

func TestCmdIssueCreate_CustomFieldNameFilteredByType(t *testing.T) {
	var posted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/projects/17.json":
			_, _ = w.Write([]byte(`{"project":{"id":17,"identifier":"demo","name":"Demo"}}`))
		case "/custom_fields.json":
			_, _ = w.Write([]byte(`{"custom_fields":[
{"id":1,"name":"Severity","customized_type":"issue","field_format":"string"},
{"id":2,"name":"Severity","customized_type":"time_entry","field_format":"string"}]}`))
		case "/issues.json":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &posted)
			_, _ = w.Write([]byte(`{"issue":{"id":1,"subject":"test","project":{"id":17,"name":"Demo"}}}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := NewCmdCreate(f)
	cmd.SetArgs([]string{"--project", "17", "--subject", "test", "--custom-field", "Severity=High", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	if got := createdIssueCFVs(t, posted); !reflect.DeepEqual(got, map[string]any{"1": "High"}) {
		t.Errorf("custom_field_values = %v, want key 1", got)
	}
}

func TestIssueCreate_RepeatedCustomFieldValidationHint(t *testing.T) {
	srv := newHintServer(t, http.StatusUnprocessableEntity, http.StatusCreated, validationBody)
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	err := runIssueCmd(t, f, "create", "--project", "demo", "--subject", "x",
		"--custom-field", "5=a", "--custom-field", "5=b")
	if err == nil {
		t.Fatal("expected error")
	}
	hint := cmdutil.ErrorHint(err)
	if !strings.Contains(hint, "multi-value") {
		t.Errorf("hint lacks multi-value text: %q", hint)
	}
	if !strings.Contains(hint, "redmine projects get demo") {
		t.Errorf("hint lost the lookup text: %q", hint)
	}

	err = runIssueCmd(t, f, "create", "--project", "demo", "--subject", "x", "--custom-field", "5=a")
	if err == nil {
		t.Fatal("expected error")
	}
	if hint := cmdutil.ErrorHint(err); strings.Contains(hint, "multi-value") {
		t.Errorf("hint mentions multi-value without repeated keys: %q", hint)
	}
}
