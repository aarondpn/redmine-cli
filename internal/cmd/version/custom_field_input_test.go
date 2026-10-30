package version

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

// customFieldServer serves /custom_fields.json with a same-named issue and
// version field and records the body of the one write request.
func customFieldServer(t *testing.T, wantMethod, wantPath, replyBody string, posted *map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/custom_fields.json" {
			_, _ = w.Write([]byte(`{"custom_fields":[
{"id":1,"name":"Region","customized_type":"issue","field_format":"string"},
{"id":2,"name":"Region","customized_type":"version","field_format":"list","multiple":true}]}`))
			return
		}
		if r.Method == http.MethodGet && r.URL.Path == "/projects/demo.json" {
			_, _ = w.Write([]byte(`{"project":{"id":1,"identifier":"demo","name":"Demo"}}`))
			return
		}
		if r.Method != wantMethod || r.URL.Path != wantPath {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, posted)
		if replyBody == "" {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(replyBody))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func assertCustomFieldValues(t *testing.T, posted map[string]any, key string, want map[string]any) {
	t.Helper()
	obj, ok := posted[key].(map[string]any)
	if !ok {
		t.Fatalf("body lacks %q: %v", key, posted)
	}
	if got := obj["custom_field_values"]; !reflect.DeepEqual(got, want) {
		t.Errorf("custom_field_values = %v, want %v", got, want)
	}
}

func TestVersionCreate_CustomFields(t *testing.T) {
	var posted map[string]any
	srv := customFieldServer(t, "POST", "/projects/demo/versions.json", `{"version":{"id":1,"name":"v"}}`, &posted)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionCreate(f)
	cmd.SetArgs([]string{"--project", "demo", "--name", "v", "--custom-field", "3=a", "--custom-field", "3=b c", "--custom-field", "Region=east", "--custom-field", "Region=west", "--custom-field", "4="})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertCustomFieldValues(t, posted, "version", map[string]any{
		"2": []any{"east", "west"}, "3": []any{"a", "b c"}, "4": "",
	})
}

func TestVersionUpdate_CustomFields(t *testing.T) {
	var posted map[string]any
	srv := customFieldServer(t, "PUT", "/versions/9.json", ``, &posted)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionUpdate(f)
	cmd.SetArgs([]string{"9", "--custom-field", "3=a", "--custom-field", "3=b c", "--custom-field", "Region=east", "--custom-field", "Region=west", "--custom-field", "4="})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertCustomFieldValues(t, posted, "version", map[string]any{
		"2": []any{"east", "west"}, "3": []any{"a", "b c"}, "4": "",
	})
}

// A repeated key that the server rejects gets the multi-value hint.
func TestVersionUpdate_RepeatedCustomFieldHint(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnprocessableEntity)
		_, _ = w.Write([]byte(`{"errors":["Release Manager is invalid"]}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdVersionUpdate(f)
	cmd.SetArgs([]string{"9", "--custom-field", "3=a", "--custom-field", "3=b"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if hint := cmdutil.ErrorHint(err); !strings.Contains(hint, "multi-value") {
		t.Errorf("hint = %q, want it to mention multi-value", hint)
	}
}
