package user

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/testutil"
)

func TestUserList_SortAndFilter(t *testing.T) {
	rec := testutil.NewQueryRecorder(t, `{"users":[],"total_count":0}`)
	f := testutil.NewFactory(t, rec.URL)
	cmd := newCmdUserList(f)
	cmd.SetArgs([]string{
		"--sort", "created_on:desc",
		"--filter", "f[]=status",
		"--filter", "f[]=login",
		"--filter", "op[status]=*",
		"--filter", "op[login]=~",
		"--filter", "v[login][]=smith",
		"--output", "json",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	q := rec.Last(t)
	testutil.WantQueryValues(t, q, "sort", "created_on:desc")
	testutil.WantQueryValues(t, q, "f[]", "status", "login")
	testutil.WantQueryValues(t, q, "op[status]", "*")
	testutil.WantQueryValues(t, q, "op[login]", "~")
	testutil.WantQueryValues(t, q, "v[login][]", "smith")
	if q.Has("status") {
		t.Errorf("status sent without --status: %s", q.Encode())
	}
}

func TestUserList_StatusAll(t *testing.T) {
	rec := testutil.NewQueryRecorder(t, `{"users":[],"total_count":0}`)
	f := testutil.NewFactory(t, rec.URL)
	cmd := newCmdUserList(f)
	cmd.SetArgs([]string{"--status", "all", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if n := len(rec.Queries()); n != 1 {
		t.Fatalf("requests = %d, want 1", n)
	}
	testutil.WantQueryValues(t, rec.Last(t), "status", "")
}

func TestUserList_StatusAllFallsBackToStarOnRedmine6(t *testing.T) {
	var (
		mu       sync.Mutex
		statuses []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		statuses = append(statuses, r.URL.Query().Get("status"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("status") == "" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"errors":["Status cannot be blank"]}`))
			return
		}
		_, _ = w.Write([]byte(`{"users":[` + userJSON + `],"total_count":1}`))
	}))
	defer srv.Close()

	f := testutil.NewFactory(t, srv.URL)
	cmd := newCmdUserList(f)
	cmd.SetArgs([]string{"--status", "all", "--output", "json"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(statuses, []string{"", "*"}) {
		t.Errorf("status values sent = %q, want empty then *", statuses)
	}
	if !strings.Contains(testutil.Stdout(f), "admin") {
		t.Errorf("stdout = %s, want the listed user", testutil.Stdout(f))
	}
}

func TestUserList_StatusHelp(t *testing.T) {
	cmd := newCmdUserList(testutil.NewFactory(t, "http://127.0.0.1:1"))
	if usage := cmd.Flags().Lookup("status").Usage; !strings.Contains(usage, "all") {
		t.Errorf("--status usage = %q, want it to mention all", usage)
	}
	if !strings.Contains(cmd.Long, "Redmine 6.0+") || !strings.Contains(cmd.Long, "5.1+") {
		t.Errorf("long help does not document the per-version behaviour: %q", cmd.Long)
	}
	if usage := cmd.Flags().Lookup("sort").Usage; usage != "Sort field (e.g., created_on:desc)" {
		t.Errorf("--sort usage = %q", usage)
	}
}
