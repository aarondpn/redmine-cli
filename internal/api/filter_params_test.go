package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/debug"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// queryRecorder answers every list request with an empty collection of key
// and records the decoded query of each request.
type queryRecorder struct {
	mu      sync.Mutex
	queries []url.Values
}

func (q *queryRecorder) client(t *testing.T, key string) *Client {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q.mu.Lock()
		q.queries = append(q.queries, r.URL.Query())
		q.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"` + key + `":[],"total_count":0}`))
	}))
	t.Cleanup(ts.Close)
	return NewTestClient(ts.Client(), ts.URL, debug.New(nil))
}

func (q *queryRecorder) last(t *testing.T) url.Values {
	t.Helper()
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.queries) == 0 {
		t.Fatal("no request recorded")
	}
	return q.queries[len(q.queries)-1]
}

func assertQueryValues(t *testing.T, got url.Values, key string, want ...string) {
	t.Helper()
	if !slices.Equal(got[key], want) {
		t.Errorf("%s = %q, want %q (query %s)", key, got[key], want, got.Encode())
	}
}

func TestApplyFilterParams(t *testing.T) {
	params := url.Values{}
	params.Set("status_id", "open")
	params.Set("project_id", "demo")
	applyFilterParams(params, models.FilterParams{
		"status_id": {"open|closed"},
		"f[]":       {"status_id", "created_on"},
		"cf_5":      {""},
		"empty":     {},
	})
	assertQueryValues(t, params, "status_id", "open|closed")
	assertQueryValues(t, params, "project_id", "demo")
	assertQueryValues(t, params, "f[]", "status_id", "created_on")
	assertQueryValues(t, params, "cf_5", "")
	assertQueryValues(t, params, "empty", "")
}

func TestApplyFilterParams_DoesNotAliasInput(t *testing.T) {
	extra := models.FilterParams{"f[]": {"a", "b"}}
	params := url.Values{}
	applyFilterParams(params, extra)
	params["f[]"][0] = "changed"
	if extra["f[]"][0] != "a" {
		t.Fatalf("input mutated: %v", extra)
	}
}

func TestIssueService_List_ExtraParamsOverrideAndRepeat(t *testing.T) {
	var rec queryRecorder
	c := rec.client(t, "issues")
	_, _, err := c.Issues.List(context.Background(), models.IssueFilter{
		StatusID: "open",
		ExtraParams: models.FilterParams{
			"status_id":        {"*"},
			"f[]":              {"status_id", "created_on"},
			"op[created_on]":   {">="},
			"v[created_on][]":  {"2025-01-01"},
			"op[status_id]":    {"*"},
			"v[status_id][]":   {""},
			"cf_5":             {"Critical"},
			"issue.tracker_id": {"1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	q := rec.last(t)
	assertQueryValues(t, q, "status_id", "*")
	assertQueryValues(t, q, "f[]", "status_id", "created_on")
	assertQueryValues(t, q, "op[created_on]", ">=")
	assertQueryValues(t, q, "v[created_on][]", "2025-01-01")
	assertQueryValues(t, q, "cf_5", "Critical")
}

func TestTimeEntryService_List_SortQueryAndExtraParams(t *testing.T) {
	var rec queryRecorder
	c := rec.client(t, "time_entries")
	_, _, err := c.TimeEntries.List(context.Background(), models.TimeEntryFilter{
		ProjectID: "demo",
		QueryID:   7,
		Sort:      "spent_on:desc",
		ExtraParams: models.FilterParams{
			"cf_3":             {"Onsite"},
			"spent_on":         {"><2026-01-01|2026-01-31"},
			"issue.tracker_id": {"1"},
			"v[hours][]":       {"1", "2"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	q := rec.last(t)
	assertQueryValues(t, q, "project_id", "demo")
	assertQueryValues(t, q, "query_id", "7")
	assertQueryValues(t, q, "sort", "spent_on:desc")
	assertQueryValues(t, q, "cf_3", "Onsite")
	assertQueryValues(t, q, "spent_on", "><2026-01-01|2026-01-31")
	assertQueryValues(t, q, "issue.tracker_id", "1")
	assertQueryValues(t, q, "v[hours][]", "1", "2")
}

func TestTimeEntryService_List_OmitsUnsetSortAndQuery(t *testing.T) {
	var rec queryRecorder
	c := rec.client(t, "time_entries")
	if _, _, err := c.TimeEntries.List(context.Background(), models.TimeEntryFilter{}); err != nil {
		t.Fatal(err)
	}
	q := rec.last(t)
	for _, key := range []string{"sort", "query_id"} {
		if q.Has(key) {
			t.Errorf("%s sent without being set: %s", key, q.Encode())
		}
	}
}

func TestProjectService_List_StatusSortAndExtraParams(t *testing.T) {
	var rec queryRecorder
	c := rec.client(t, "projects")
	_, _, err := c.Projects.List(context.Background(), models.ProjectFilter{
		Status:      "1|5",
		Sort:        "name:desc",
		ExtraParams: models.FilterParams{"name": {"~web"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	q := rec.last(t)
	assertQueryValues(t, q, "status", "1|5")
	assertQueryValues(t, q, "sort", "name:desc")
	assertQueryValues(t, q, "name", "~web")
}

func TestProjectService_List_NoStatusByDefault(t *testing.T) {
	var rec queryRecorder
	c := rec.client(t, "projects")
	if _, _, err := c.Projects.List(context.Background(), models.ProjectFilter{}); err != nil {
		t.Fatal(err)
	}
	if q := rec.last(t); q.Has("status") || q.Has("sort") {
		t.Errorf("unexpected status/sort in %s", q.Encode())
	}
}

func TestUserService_List_SortAndExtraParams(t *testing.T) {
	var rec queryRecorder
	c := rec.client(t, "users")
	_, _, err := c.Users.List(context.Background(), models.UserFilter{
		Status: "locked",
		Sort:   "created_on:desc",
		ExtraParams: models.FilterParams{
			"f[]":        {"status", "login"},
			"op[status]": {"*"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	q := rec.last(t)
	assertQueryValues(t, q, "status", "3")
	assertQueryValues(t, q, "sort", "created_on:desc")
	assertQueryValues(t, q, "f[]", "status", "login")
	assertQueryValues(t, q, "op[status]", "*")
}

// usersStatusServer answers /users.json like Redmine 6.0+ when reject is set:
// an empty status= is a 422, status=* lists every user.
func usersStatusServer(t *testing.T, reject bool, statuses *[]string) *Client {
	t.Helper()
	var mu sync.Mutex
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		mu.Lock()
		if q.Has("status") {
			*statuses = append(*statuses, q.Get("status"))
		} else {
			*statuses = append(*statuses, "<unset>")
		}
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		if reject && q.Has("status") && q.Get("status") == "" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write([]byte(`{"errors":["Status cannot be blank"]}`))
			return
		}
		_, _ = w.Write([]byte(`{"users":[{"id":1,"login":"admin"},{"id":5,"login":"locked"}],"total_count":2}`))
	}))
	t.Cleanup(ts.Close)
	return NewTestClient(ts.Client(), ts.URL, debug.New(nil))
}

func TestUserService_List_StatusAllSendsEmptyStatus(t *testing.T) {
	var statuses []string
	c := usersStatusServer(t, false, &statuses)
	users, total, err := c.Users.List(context.Background(), models.UserFilter{Status: UserStatusAll})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 || total != 2 {
		t.Fatalf("got %d users (total %d), want 2", len(users), total)
	}
	if !slices.Equal(statuses, []string{""}) {
		t.Errorf("status values sent = %q, want one empty status", statuses)
	}
}

func TestUserService_List_StatusAllRetriesWithStarOn422(t *testing.T) {
	var statuses []string
	c := usersStatusServer(t, true, &statuses)
	users, _, err := c.Users.List(context.Background(), models.UserFilter{Status: UserStatusAll})
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 2 {
		t.Fatalf("got %d users, want 2", len(users))
	}
	if !slices.Equal(statuses, []string{"", "*"}) {
		t.Errorf("status values sent = %q, want empty then *", statuses)
	}
}

func TestUserService_List_StatusAllNoRetryWhenFilterOverridesStatus(t *testing.T) {
	var statuses []string
	c := usersStatusServer(t, true, &statuses)
	_, _, err := c.Users.List(context.Background(), models.UserFilter{
		Status:      UserStatusAll,
		ExtraParams: models.FilterParams{"status": {""}},
	})
	// A raw status filter wins over --status all, so its 422 is the user's
	// to see and no status=* retry replaces the value they asked for.
	if err == nil {
		t.Fatal("expected the 422 to surface")
	}
	if !slices.Equal(statuses, []string{""}) {
		t.Errorf("status values sent = %q, want one empty status", statuses)
	}
}

func TestUserService_List_OtherStatusNoRetry(t *testing.T) {
	var statuses []string
	c := usersStatusServer(t, true, &statuses)
	if _, _, err := c.Users.List(context.Background(), models.UserFilter{}); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(statuses, []string{"<unset>"}) {
		t.Errorf("status values sent = %q, want none", statuses)
	}
}

func TestGroupService_List_Builtin(t *testing.T) {
	var rec queryRecorder
	c := rec.client(t, "groups")
	if _, _, err := c.Groups.List(context.Background(), models.GroupFilter{}); err != nil {
		t.Fatal(err)
	}
	if rec.last(t).Has("builtin") {
		t.Errorf("builtin sent by default: %s", rec.last(t).Encode())
	}
	if _, _, err := c.Groups.List(context.Background(), models.GroupFilter{Builtin: true}); err != nil {
		t.Fatal(err)
	}
	assertQueryValues(t, rec.last(t), "builtin", "1")
}

func TestQueryService_ListOfType(t *testing.T) {
	var rec queryRecorder
	c := rec.client(t, "queries")
	if _, _, err := c.Queries.List(context.Background(), 0, 0); err != nil {
		t.Fatal(err)
	}
	if rec.last(t).Has("type") {
		t.Errorf("type sent for issue queries: %s", rec.last(t).Encode())
	}
	if _, _, err := c.Queries.ListOfType(context.Background(), "TimeEntryQuery", 0, 0); err != nil {
		t.Fatal(err)
	}
	assertQueryValues(t, rec.last(t), "type", "TimeEntryQuery")
}
