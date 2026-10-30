package resolver

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/config"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
)

const customFieldsFixture = `{"custom_fields":[
 {"id":1,"name":"Severity","customized_type":"issue","field_format":"list"},
 {"id":2,"name":"Severity","customized_type":"time_entry","field_format":"string"},
 {"id":3,"name":"Tags","customized_type":"issue","field_format":"list","multiple":true},
 {"id":4,"name":"Owner","customized_type":"version","field_format":"string"}
]}`

func newCustomFieldClient(t *testing.T, status int) (*api.Client, *atomic.Int32) {
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
			_, _ = w.Write([]byte(customFieldsFixture))
		} else {
			_, _ = w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	client, err := api.NewClient(&config.Config{Server: srv.URL, APIKey: "k", AuthMethod: "apikey"}, debug.New(nil))
	if err != nil {
		t.Fatal(err)
	}
	return client, &hits
}

func TestResolveCustomFieldsForType_NumericSkipsFetch(t *testing.T) {
	client, hits := newCustomFieldClient(t, http.StatusOK)
	got, err := ResolveCustomFieldsForType(context.Background(), client, []string{"5", "9"}, "issue")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != 5 || got[1].ID != 9 {
		t.Errorf("got %+v", got)
	}
	if hits.Load() != 0 {
		t.Errorf("fetched %d times, want 0", hits.Load())
	}
}

func TestResolveCustomFieldsForType_FiltersByType(t *testing.T) {
	client, _ := newCustomFieldClient(t, http.StatusOK)
	ctx := context.Background()
	for typ, want := range map[string]int{"time_entry": 2, "issue": 1} {
		got, err := ResolveCustomFieldsForType(ctx, client, []string{"Severity"}, typ)
		if err != nil {
			t.Fatalf("%s: %v", typ, err)
		}
		if got[0].ID != want {
			t.Errorf("%s: ID = %d, want %d", typ, got[0].ID, want)
		}
	}
	got, err := ResolveCustomFieldsForType(ctx, client, []string{"tags"}, "issue")
	if err != nil {
		t.Fatal(err)
	}
	if got[0].ID != 3 || !got[0].Multiple {
		t.Errorf("case-insensitive match lost the definition: %+v", got[0])
	}
}

func TestResolveCustomFieldsForType_OtherType(t *testing.T) {
	client, _ := newCustomFieldClient(t, http.StatusOK)
	_, err := ResolveCustomFieldsForType(context.Background(), client, []string{"Owner"}, "issue")
	if err == nil {
		t.Fatal("expected error")
	}
	for _, want := range []string{"custom field of type version", "numeric ID", "issue custom field"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestResolveCustomFieldsForType_NoMatchListsSameTypeOnly(t *testing.T) {
	client, _ := newCustomFieldClient(t, http.StatusOK)
	_, err := ResolveCustomFieldsForType(context.Background(), client, []string{"Nope"}, "issue")
	if err == nil || !strings.Contains(err.Error(), "no match found") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(err.Error(), "(ID: 1)") || !strings.Contains(err.Error(), "(ID: 3)") {
		t.Errorf("missing issue fields: %v", err)
	}
	if strings.Contains(err.Error(), "(ID: 2)") || strings.Contains(err.Error(), "(ID: 4)") {
		t.Errorf("listed other types: %v", err)
	}
}

func TestResolveCustomFieldsForType_SingleFetch(t *testing.T) {
	client, hits := newCustomFieldClient(t, http.StatusOK)
	if _, err := ResolveCustomFieldsForType(context.Background(), client, []string{"Severity", "Tags", "3"}, "issue"); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Errorf("fetched %d times, want 1", hits.Load())
	}
}

func TestResolveCustomFieldsForType_Forbidden(t *testing.T) {
	client, _ := newCustomFieldClient(t, http.StatusForbidden)
	_, err := ResolveCustomFieldsForType(context.Background(), client, []string{"Severity"}, "issue")
	if !IsNameResolutionPermissionError(err) {
		t.Fatalf("err = %v, want permission error", err)
	}
	if !strings.Contains(err.Error(), "issue custom field") {
		t.Errorf("message %q lacks label", err)
	}
}

func TestResolveCustomFieldNames_ByType(t *testing.T) {
	client, _ := newCustomFieldClient(t, http.StatusOK)
	ctx := context.Background()
	got, err := ResolveCustomFieldNames(ctx, client, []string{"Severity", "", "3"}, "issue")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []int{1, 3}) {
		t.Errorf("got %v, want [1 3]", got)
	}
	if _, err := ResolveCustomFieldNames(ctx, client, []string{"Owner"}, "issue"); err == nil || !strings.Contains(err.Error(), "of type version") {
		t.Errorf("Owner is a version field and must be reported as such, got %v", err)
	}
	if _, err := ResolveCustomFieldNames(ctx, client, []string{"Severity"}, ""); err == nil || !strings.Contains(err.Error(), "multiple") {
		t.Errorf("untyped Severity should be ambiguous, got %v", err)
	}
}

func TestCustomFieldLabel(t *testing.T) {
	for in, want := range map[string]string{
		"time_entry": "time entry custom field",
		"issue":      "issue custom field",
		"":           "custom field",
	} {
		if got := CustomFieldLabel(in); got != want {
			t.Errorf("CustomFieldLabel(%q) = %q, want %q", in, got, want)
		}
	}
}
