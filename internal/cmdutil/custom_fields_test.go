package cmdutil_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

const cfFixture = `{"custom_fields":[
 {"id":1,"name":"Severity","customized_type":"issue","field_format":"list"},
 {"id":2,"name":"Severity","customized_type":"time_entry","field_format":"string"},
 {"id":3,"name":"Tags","customized_type":"issue","field_format":"list","multiple":true},
 {"id":4,"name":"Notes","customized_type":"issue","field_format":"string"}
]}`

// newCFClient serves /custom_fields.json with the given status and counts hits.
func newCFClient(t *testing.T, status int) (*api.Client, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/custom_fields.json" {
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		hits.Add(1)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_, _ = w.Write([]byte(cfFixture))
		} else {
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	f := testutil.NewFactory(t, srv.URL)
	client, err := f.ApiClient()
	if err != nil {
		t.Fatal(err)
	}
	return client, &hits
}

func TestParseCustomFieldPairs(t *testing.T) {
	ok := []struct {
		raw  string
		want cmdutil.CustomFieldPair
	}{
		{"5=a", cmdutil.CustomFieldPair{Key: "5", Value: "a"}},
		{"2=Value for field 2", cmdutil.CustomFieldPair{Key: "2", Value: "Value for field 2"}},
		{"5=a=b", cmdutil.CustomFieldPair{Key: "5", Value: "a=b"}},
		{" 5 = x", cmdutil.CustomFieldPair{Key: "5", Value: " x"}},
		{"5=", cmdutil.CustomFieldPair{Key: "5", Value: ""}},
	}
	for _, tt := range ok {
		got, err := cmdutil.ParseCustomFieldPairs([]string{tt.raw})
		if err != nil {
			t.Errorf("%q: %v", tt.raw, err)
			continue
		}
		if len(got) != 1 || got[0] != tt.want {
			t.Errorf("%q: got %+v, want %+v", tt.raw, got, tt.want)
		}
	}

	if _, err := cmdutil.ParseCustomFieldPairs([]string{"5"}); err == nil || !strings.Contains(err.Error(), "must be key=value") {
		t.Errorf("missing '=': %v", err)
	}
	if _, err := cmdutil.ParseCustomFieldPairs([]string{"=x"}); err == nil || !strings.Contains(err.Error(), "empty key") {
		t.Errorf("empty key: %v", err)
	}

	got, err := cmdutil.ParseCustomFieldPairs([]string{"b=1", "a=2", "b=3"})
	if err != nil {
		t.Fatal(err)
	}
	want := []cmdutil.CustomFieldPair{{Key: "b", Value: "1"}, {Key: "a", Value: "2"}, {Key: "b", Value: "3"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("order not preserved: %+v", got)
	}
}

func TestParseCustomFieldValues_NumericKeysNeverFetch(t *testing.T) {
	client, hits := newCFClient(t, http.StatusOK)
	got, err := cmdutil.ParseCustomFieldValues(context.Background(), client,
		[]string{"5=a", "5=b", "007=c", "6=x"}, models.CustomizedTypeIssue)
	if err != nil {
		t.Fatal(err)
	}
	want := models.CustomFieldValues{"5": {"a", "b"}, "7": {"c"}, "6": {"x"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if hits.Load() != 0 {
		t.Errorf("fetched %d times, want 0", hits.Load())
	}
}

func TestParseCustomFieldValues_InvalidID(t *testing.T) {
	client, _ := newCFClient(t, http.StatusOK)
	for _, raw := range []string{"0=x", "-1=x"} {
		_, err := cmdutil.ParseCustomFieldValues(context.Background(), client, []string{raw}, models.CustomizedTypeIssue)
		if err == nil || !strings.Contains(err.Error(), "invalid custom field ID") {
			t.Errorf("%s: err = %v", raw, err)
		}
	}
}

func TestParseCustomFieldValues_NamesMergeWithIDs(t *testing.T) {
	client, hits := newCFClient(t, http.StatusOK)
	got, err := cmdutil.ParseCustomFieldValues(context.Background(), client,
		[]string{"Tags=a", "3=b", "tags=c"}, models.CustomizedTypeIssue)
	if err != nil {
		t.Fatal(err)
	}
	if want := (models.CustomFieldValues{"3": {"a", "b", "c"}}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
	if hits.Load() != 1 {
		t.Errorf("fetched %d times, want 1", hits.Load())
	}
}

func TestParseCustomFieldValues_NameFilteredByType(t *testing.T) {
	client, _ := newCFClient(t, http.StatusOK)
	for typ, key := range map[string]string{models.CustomizedTypeTimeEntry: "2", models.CustomizedTypeIssue: "1"} {
		got, err := cmdutil.ParseCustomFieldValues(context.Background(), client, []string{"Severity=High"}, typ)
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if _, ok := got[key]; !ok || len(got) != 1 {
			t.Errorf("%s: got %v, want key %s", typ, got, key)
		}
	}
}

func TestParseCustomFieldValues_MultipleValidation(t *testing.T) {
	client, _ := newCFClient(t, http.StatusOK)
	ctx := context.Background()

	_, err := cmdutil.ParseCustomFieldValues(ctx, client, []string{"Notes=a", "Notes=b"}, models.CustomizedTypeIssue)
	if err == nil || !strings.Contains(err.Error(), "does not accept multiple values") {
		t.Errorf("single-value field repeated: %v", err)
	}
	if _, err := cmdutil.ParseCustomFieldValues(ctx, client, []string{"Tags=a", "Tags=b"}, models.CustomizedTypeIssue); err != nil {
		t.Errorf("multi-value field repeated: %v", err)
	}

	_, err = cmdutil.ParseCustomFieldValues(ctx, client, []string{"5=", "5=a"}, models.CustomizedTypeIssue)
	if err == nil || !strings.Contains(err.Error(), "cannot be combined") {
		t.Errorf("clear combined with value: %v", err)
	}
}

func TestParseCustomFieldValues_Clear(t *testing.T) {
	client, _ := newCFClient(t, http.StatusOK)
	got, err := cmdutil.ParseCustomFieldValues(context.Background(), client, []string{"5="}, models.CustomizedTypeIssue)
	if err != nil {
		t.Fatal(err)
	}
	if want := (models.CustomFieldValues{"5": {""}}); !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}
	repeated, err := cmdutil.ParseCustomFieldValues(context.Background(), client, []string{"5=", "5="}, models.CustomizedTypeIssue)
	if err != nil {
		t.Fatalf("repeated clear: %v", err)
	}
	if want := (models.CustomFieldValues{"5": {""}}); !reflect.DeepEqual(repeated, want) {
		t.Errorf("repeated clear: got %v", repeated)
	}
	data, _ := json.Marshal(got)
	if string(data) != `{"5":""}` {
		t.Errorf("marshals to %s", data)
	}
}

func TestParseCustomFieldValues_ForbiddenLookup(t *testing.T) {
	client, _ := newCFClient(t, http.StatusForbidden)
	_, err := cmdutil.ParseCustomFieldValues(context.Background(), client, []string{"Severity=x"}, models.CustomizedTypeIssue)
	if !resolver.IsNameResolutionPermissionError(err) {
		t.Fatalf("err = %v, want permission error", err)
	}
	if hint := cmdutil.ErrorHint(err); !strings.Contains(hint, "--jq '.custom_fields'") {
		t.Errorf("hint = %q", hint)
	}
}

func TestParseCustomFieldValues_Nil(t *testing.T) {
	client, _ := newCFClient(t, http.StatusOK)
	got, err := cmdutil.ParseCustomFieldValues(context.Background(), client, nil, models.CustomizedTypeIssue)
	if got != nil || err != nil {
		t.Errorf("got %v, %v", got, err)
	}
}

func TestHasRepeatedCustomFieldKey(t *testing.T) {
	tests := []struct {
		raws []string
		want bool
	}{
		{[]string{"5=a", "5=b"}, true},
		{[]string{"5=a", "05=b"}, true},
		{[]string{"Tags=a", "tags=b"}, true},
		{[]string{"5=a", "6=b"}, false},
		{[]string{"5=a", "oops", "5=b"}, false},
	}
	for _, tt := range tests {
		if got := cmdutil.HasRepeatedCustomFieldKey(tt.raws); got != tt.want {
			t.Errorf("%v: got %v, want %v", tt.raws, got, tt.want)
		}
	}
}

func TestCustomFieldValidationHint(t *testing.T) {
	if got := cmdutil.CustomFieldValidationHint([]string{"5=a", "6=b"}); got != "" {
		t.Errorf("hint without repeats = %q", got)
	}
	if got := cmdutil.CustomFieldValidationHint([]string{"5=a", "5=b"}); !strings.Contains(got, "multi-value") {
		t.Errorf("hint with repeats = %q", got)
	}
}

func TestWithValidationHint(t *testing.T) {
	validation := &api.APIError{StatusCode: http.StatusUnprocessableEntity}
	if got := cmdutil.ErrorHint(cmdutil.WithValidationHint(validation, "one", "", "two")); got != "one two" {
		t.Errorf("joined hint = %q", got)
	}
	if got := cmdutil.ErrorHint(cmdutil.WithValidationHint(validation, "", "")); got != "" {
		t.Errorf("all-empty hints produced %q", got)
	}

	notFound := &api.APIError{StatusCode: http.StatusNotFound}
	if got := cmdutil.WithValidationHint(notFound, "x"); got != error(notFound) {
		t.Errorf("404 was wrapped: %v", got)
	}
	plain := errors.New("boom")
	if got := cmdutil.WithValidationHint(plain, "x"); got != plain {
		t.Errorf("plain error was wrapped: %v", got)
	}
}

func TestAddCustomFieldFlag(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	f := testutil.NewFactory(t, srv.URL)

	cmd := &cobra.Command{Use: "x"}
	var raws []string
	cmdutil.AddCustomFieldFlag(cmd, f, &raws, "Time entry custom field", models.CustomizedTypeTimeEntry)

	fl := cmd.Flags().Lookup("custom-field")
	if fl == nil || fl.Value.Type() != "stringArray" {
		t.Fatalf("flag = %+v", fl)
	}
	want := "Time entry custom field value as name=value or id=value (repeatable; repeat a field to set several values of a multi-value field, name= clears it)"
	if fl.Usage != want {
		t.Errorf("usage = %q", fl.Usage)
	}
	if fn, ok := cmd.GetFlagCompletionFunc("custom-field"); !ok || fn == nil {
		t.Error("no completion func registered")
	}
}
