package time

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestTimeList_SortQueryIDAndFilter(t *testing.T) {
	rec := testutil.NewQueryRecorder(t, `{"time_entries":[],"total_count":0}`)
	f := testutil.NewFactory(t, rec.URL)
	cmd := newCmdTimeList(f)
	cmd.SetArgs([]string{
		"--sort", "spent_on:desc",
		"--query-id", "7",
		"--filter", "cf_3=Onsite",
		"--filter", "spent_on=><2026-01-01|2026-01-31",
		"--filter", "f[]=hours",
		"--filter", "f[]=spent_on",
		"--filter", "op[hours]=>=",
		"--output", "json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if n := len(rec.Queries()); n != 1 {
		t.Fatalf("requests = %d, want 1", n)
	}
	q := rec.Last(t)
	testutil.WantQueryValues(t, q, "sort", "spent_on:desc")
	testutil.WantQueryValues(t, q, "query_id", "7")
	testutil.WantQueryValues(t, q, "cf_3", "Onsite")
	testutil.WantQueryValues(t, q, "spent_on", "><2026-01-01|2026-01-31")
	testutil.WantQueryValues(t, q, "f[]", "hours", "spent_on")
	testutil.WantQueryValues(t, q, "op[hours]", ">=")
}

func TestTimeList_NoSortOrQueryByDefault(t *testing.T) {
	rec := testutil.NewQueryRecorder(t, `{"time_entries":[],"total_count":0}`)
	f := testutil.NewFactory(t, rec.URL)
	cmd := newCmdTimeList(f)
	cmd.SetArgs([]string{"--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if q := rec.Last(t); q.Has("sort") || q.Has("query_id") {
		t.Errorf("unexpected sort/query_id: %s", q.Encode())
	}
}

func TestTimeList_QueryByNameResolvesTimeEntryQueries(t *testing.T) {
	var (
		mu        sync.Mutex
		queryType string
		entries   url.Values
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/queries.json":
			queryType = r.URL.Query().Get("type")
			_, _ = w.Write([]byte(`{"queries":[{"id":7,"name":"Spent time","is_public":true,"project_id":null}],"total_count":1}`))
		case "/time_entries.json":
			entries = r.URL.Query()
			_, _ = w.Write([]byte(`{"time_entries":[],"total_count":0}`))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdTimeList(f)
	cmd.SetArgs([]string{"--query", "Spent time", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if queryType != "TimeEntryQuery" {
		t.Errorf("queries type = %q, want TimeEntryQuery", queryType)
	}
	testutil.WantQueryValues(t, entries, "query_id", "7")
}

func TestTimeList_QueryAndFilterValidation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request expected, got %s", r.URL)
	}))
	defer srv.Close()

	for name, args := range map[string][]string{
		"mutually exclusive": {"--query", "Spent time", "--query-id", "7"},
		"query-id zero":      {"--query-id", "0"},
		"filter without =":   {"--filter", "nokey"},
	} {
		t.Run(name, func(t *testing.T) {
			f := testutil.NewFactory(t, srv.URL)
			cmd := newCmdTimeList(f)
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
			cmd.SetArgs(append(args, "--output", "json"))
			if err := cmd.Execute(); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestTimeList_FlagHelp(t *testing.T) {
	cmd := newCmdTimeList(testutil.NewFactory(t, "http://127.0.0.1:1"))
	if usage := cmd.Flags().Lookup("sort").Usage; usage != "Sort field (e.g., spent_on:desc)" {
		t.Errorf("--sort usage = %q", usage)
	}
	if usage := cmd.Flags().Lookup("filter").Usage; !strings.Contains(usage, "--filter cf_3=Onsite") {
		t.Errorf("--filter usage lacks a time entry example: %q", usage)
	}
	if cmd.Flags().Lookup("query") == nil || cmd.Flags().Lookup("query-id") == nil {
		t.Error("--query / --query-id not registered")
	}
}
