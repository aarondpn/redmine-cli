package cmdutil_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func TestParseFilterFlag(t *testing.T) {
	got, err := cmdutil.ParseFilterFlag([]string{
		"cf_5=Critical",
		"subject=~a=b",              // value keeps later '='
		" created_on =>=2025-01-01", // key trimmed, value verbatim
		"status_id=open",
		"status_id=*", // non-[] key: last value wins
		"f[]=status_id",
		"f[]=created_on", // [] key: values collect in flag order
		"v[status_id][]=",
		"empty=",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := models.FilterParams{
		"cf_5":           {"Critical"},
		"subject":        {"~a=b"},
		"created_on":     {">=2025-01-01"},
		"status_id":      {"*"},
		"f[]":            {"status_id", "created_on"},
		"v[status_id][]": {""},
		"empty":          {""},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ParseFilterFlag = %#v\nwant %#v", got, want)
	}
}

func TestParseFilterFlag_Empty(t *testing.T) {
	got, err := cmdutil.ParseFilterFlag(nil)
	if err != nil || got != nil {
		t.Fatalf("ParseFilterFlag(nil) = %#v, %v; want nil, nil", got, err)
	}
}

func TestParseFilterFlag_Errors(t *testing.T) {
	for raw, want := range map[string]string{
		"noequals": `--filter "noequals" must be key=value`,
		"=value":   `--filter "=value": empty key`,
		"  =value": `--filter "  =value": empty key`,
	} {
		_, err := cmdutil.ParseFilterFlag([]string{raw})
		if err == nil || err.Error() != want {
			t.Errorf("ParseFilterFlag(%q) error = %v, want %q", raw, err, want)
		}
	}
}

func TestAddFilterFlag_HelpText(t *testing.T) {
	var raws []string
	cmd := &cobra.Command{Use: "list"}
	cmdutil.AddFilterFlag(cmd, &raws, "cf_3=Onsite", "spent_on='><2026-01-01|2026-01-31'")
	flag := cmd.Flags().Lookup("filter")
	if flag == nil {
		t.Fatal("--filter not registered")
	}
	if flag.Value.Type() != "stringArray" {
		t.Errorf("type = %s, want stringArray (values may contain commas)", flag.Value.Type())
	}
	for _, want := range []string{
		"Raw Redmine filter as key=value",
		"overriding a flag that sets the same key",
		"repeat a key ending in [] to send several values",
		"Examples: --filter cf_3=Onsite, --filter spent_on='><2026-01-01|2026-01-31'",
	} {
		if !strings.Contains(flag.Usage, want) {
			t.Errorf("usage %q lacks %q", flag.Usage, want)
		}
	}
	if err := cmd.ParseFlags([]string{"--filter", "a=1,2", "--filter", "f[]=x"}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(raws, []string{"a=1,2", "f[]=x"}) {
		t.Errorf("raws = %q", raws)
	}
}

func TestResolveSavedQueryFlags(t *testing.T) {
	var gotType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/queries.json" {
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		gotType = r.URL.Query().Get("type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"queries":[{"id":7,"name":"Spent time","is_public":true,"project_id":null}],"total_count":1}`))
	}))
	defer srv.Close()
	client := api.NewTestClient(srv.Client(), srv.URL, debug.New(nil))
	ctx := context.Background()

	id, err := cmdutil.ResolveSavedQueryFlags(ctx, client, "spent TIME", 0, false, "", "TimeEntryQuery")
	if err != nil || id != 7 {
		t.Fatalf("by name = %d, %v; want 7", id, err)
	}
	if gotType != "TimeEntryQuery" {
		t.Errorf("type = %q, want TimeEntryQuery", gotType)
	}

	if id, err := cmdutil.ResolveSavedQueryFlags(ctx, client, "", 12, true, "", "TimeEntryQuery"); err != nil || id != 12 {
		t.Errorf("--query-id 12 = %d, %v", id, err)
	}
	if _, err := cmdutil.ResolveSavedQueryFlags(ctx, client, "", 0, true, "", "TimeEntryQuery"); err == nil ||
		!strings.Contains(err.Error(), "--query-id must be a positive integer") {
		t.Errorf("--query-id 0 error = %v", err)
	}
	if id, err := cmdutil.ResolveSavedQueryFlags(ctx, client, "", 0, false, "", "TimeEntryQuery"); err != nil || id != 0 {
		t.Errorf("unset = %d, %v; want 0, nil", id, err)
	}
	if _, err := cmdutil.ResolveSavedQueryFlags(ctx, client, "Nope", 0, false, "", "TimeEntryQuery"); err == nil ||
		!strings.Contains(err.Error(), "resolving query") {
		t.Errorf("unknown name error = %v", err)
	}
}
