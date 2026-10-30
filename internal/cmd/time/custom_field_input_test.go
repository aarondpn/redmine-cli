package time

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

func TestTimeLog_CustomFields(t *testing.T) {
	var posted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/time_entries.json" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &posted)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"time_entry":` + timeEntryJSON + `}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeLog(f)
	cmd.SetArgs([]string{"--issue", "10", "--hours", "1",
		"--custom-field", "3=a", "--custom-field", "3=b c", "--custom-field", "4=x"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	entry := posted["time_entry"].(map[string]any)
	want := map[string]any{"3": []any{"a", "b c"}, "4": "x"}
	if got := entry["custom_field_values"]; !reflect.DeepEqual(got, want) {
		t.Errorf("custom_field_values = %v, want %v", got, want)
	}
}

func TestTimeLog_CustomFieldByNameUsesTimeEntryType(t *testing.T) {
	var posted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/custom_fields.json":
			_, _ = w.Write([]byte(`{"custom_fields":[
{"id":1,"name":"Severity","customized_type":"issue","field_format":"string"},
{"id":2,"name":"Severity","customized_type":"time_entry","field_format":"string"}]}`))
		case "/time_entries.json":
			raw, _ := io.ReadAll(r.Body)
			_ = json.Unmarshal(raw, &posted)
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{"time_entry":` + timeEntryJSON + `}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeLog(f)
	cmd.SetArgs([]string{"--issue", "10", "--hours", "1", "--custom-field", "Severity=High"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	entry := posted["time_entry"].(map[string]any)
	want := map[string]any{"2": "High"}
	if got := entry["custom_field_values"]; !reflect.DeepEqual(got, want) {
		t.Errorf("custom_field_values = %v, want %v", got, want)
	}
}

func TestTimeLog_CustomFieldValidationHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":["Billing Code cannot be blank"]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeLog(f)
	cmd.SetArgs([]string{"--issue", "10", "--hours", "1"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if hint := cmdutil.ErrorHint(err); !strings.Contains(hint, "--custom-field") {
		t.Errorf("hint = %q, want it to mention --custom-field", hint)
	}
}

func TestTimeUpdate_CustomFields(t *testing.T) {
	var posted map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/time_entries/42.json" {
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
	cmd := newCmdTimeUpdate(f)
	cmd.SetArgs([]string{"42", "--custom-field", "3=a", "--custom-field", "3=b"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	entry := posted["time_entry"].(map[string]any)
	want := map[string]any{"3": []any{"a", "b"}}
	if got := entry["custom_field_values"]; !reflect.DeepEqual(got, want) {
		t.Errorf("custom_field_values = %v, want %v", got, want)
	}

	// Without the flag the key is absent.
	posted = nil
	cmd = newCmdTimeUpdate(testutil.NewFactory(t, srv.URL))
	cmd.SetArgs([]string{"42", "--hours", "2"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if _, ok := posted["time_entry"].(map[string]any)["custom_field_values"]; ok {
		t.Errorf("body should omit custom_field_values: %v", posted)
	}
}
