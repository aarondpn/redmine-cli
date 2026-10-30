package myaccount

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

// customFieldServer serves /custom_fields.json with a same-named issue and
// user field and records the body of the one write request.
func customFieldServer(t *testing.T, wantMethod, wantPath, replyBody string, posted *map[string]any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/custom_fields.json" {
			_, _ = w.Write([]byte(`{"custom_fields":[
{"id":1,"name":"Region","customized_type":"issue","field_format":"string"},
{"id":2,"name":"Region","customized_type":"user","field_format":"list","multiple":true}]}`))
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

func TestMyAccountUpdate_CustomFields(t *testing.T) {
	var posted map[string]any
	srv := customFieldServer(t, "PUT", "/my/account.json", ``, &posted)
	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMyAccountUpdate(f)
	cmd.SetArgs([]string{"--firstname", "X", "--custom-field", "3=a", "--custom-field", "3=b c", "--custom-field", "Region=east", "--custom-field", "Region=west", "--custom-field", "4="})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	assertCustomFieldValues(t, posted, "user", map[string]any{
		"2": []any{"east", "west"}, "3": []any{"a", "b c"}, "4": "",
	})
}

func TestMyAccount_UpdateCustomFieldNameForbidden(t *testing.T) {
	var puts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			puts++
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdMyAccountUpdate(f)
	cmd.SetArgs([]string{"--custom-field", "Skills=Go"})
	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error")
	}
	if !resolver.IsNameResolutionPermissionError(err) {
		t.Errorf("error %v is not a name resolution permission error", err)
	}
	if hint := cmdutil.ErrorHint(err); !strings.Contains(hint, "my-account get") {
		t.Errorf("hint = %q, want it to mention my-account get", hint)
	}
	if puts != 0 {
		t.Errorf("sent %d PUT requests, want none", puts)
	}
}
