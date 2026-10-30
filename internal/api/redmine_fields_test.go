package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func newFieldsTestClient(t *testing.T, body string, onReq func(r *http.Request)) *Client {
	t.Helper()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if onReq != nil {
			onReq(r)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(ts.Close)
	c := newTestClient(ts)
	c.Users = &UserService{client: c}
	c.Versions = &VersionService{client: c}
	c.Groups = &GroupService{client: c}
	c.TimeEntries = &TimeEntryService{client: c}
	c.Enumerations = &EnumerationService{client: c}
	c.Search = &SearchService{client: c}
	return c
}

func TestUserService_ListSendsIncludes(t *testing.T) {
	var got string
	c := newFieldsTestClient(t, `{"users":[{"id":1,"login":"a","auth_source":{"id":2,"name":"LDAP"}}],"total_count":1}`, func(r *http.Request) {
		got = r.URL.Query().Get("include")
	})
	users, _, err := c.Users.List(context.Background(), models.UserFilter{Includes: []string{"auth_source"}})
	if err != nil {
		t.Fatal(err)
	}
	if got != "auth_source" {
		t.Errorf("include = %q, want auth_source", got)
	}
	if users[0].AuthSource == nil || users[0].AuthSource.Name != "LDAP" {
		t.Errorf("auth_source = %+v", users[0].AuthSource)
	}
}

func TestUserService_ListOmitsIncludeByDefault(t *testing.T) {
	var has bool
	c := newFieldsTestClient(t, `{"users":[],"total_count":0}`, func(r *http.Request) {
		_, has = r.URL.Query()["include"]
	})
	if _, _, err := c.Users.List(context.Background(), models.UserFilter{}); err != nil {
		t.Fatal(err)
	}
	if has {
		t.Error("include sent without Includes")
	}
}

func TestVersionService_NormalizesCustomFields(t *testing.T) {
	c := newFieldsTestClient(t, `{"version":{"id":1,"name":"v"},"versions":[{"id":1,"name":"v"}],"total_count":1}`, nil)
	v, err := c.Versions.Get(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if v.CustomFields == nil {
		t.Error("Get: custom_fields nil, want []")
	}
	if v.EstimatedHours != nil || v.SpentHours != nil {
		t.Error("hours invented")
	}
	list, _, err := c.Versions.List(context.Background(), "demo", 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if list[0].CustomFields == nil {
		t.Error("List: custom_fields nil, want []")
	}
}

func TestGroupTimeEntryEnumerationService_NormalizeCustomFields(t *testing.T) {
	c := newFieldsTestClient(t, `{"group":{"id":1,"name":"g"},"groups":[{"id":1,"name":"g"}],"total_count":1,
	  "time_entry":{"id":1},"time_entries":[{"id":1}],
	  "time_entry_activities":[{"id":1,"name":"a"}],"issue_priorities":[{"id":1,"name":"p"}]}`, nil)
	ctx := context.Background()

	g, err := c.Groups.Get(ctx, 1, nil)
	if err != nil || g.CustomFields == nil {
		t.Errorf("group get: err=%v group=%+v", err, g)
	}
	gs, _, err := c.Groups.List(ctx, models.GroupFilter{})
	if err != nil || gs[0].CustomFields == nil {
		t.Errorf("group list: err=%v", err)
	}
	te, err := c.TimeEntries.Get(ctx, 1)
	if err != nil || te.CustomFields == nil {
		t.Errorf("time entry get: err=%v", err)
	}
	tes, _, err := c.TimeEntries.List(ctx, models.TimeEntryFilter{})
	if err != nil || tes[0].CustomFields == nil {
		t.Errorf("time entry list: err=%v", err)
	}
	acts, err := c.Enumerations.TimeEntryActivities(ctx)
	if err != nil || acts[0].CustomFields == nil {
		t.Errorf("activities: err=%v", err)
	}
	prios, err := c.Enumerations.IssuePriorities(ctx)
	if err != nil || prios[0].CustomFields == nil {
		t.Errorf("priorities: err=%v", err)
	}
}

func TestSearchService_SearchPageKeepsMeta(t *testing.T) {
	c := newFieldsTestClient(t, `{"results":[{"id":1,"title":"t"}],"total_count":7,"offset":5,"limit":1}`, nil)
	resp, err := c.Search.SearchPage(context.Background(), SearchParams{Query: "q"})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Offset == nil || *resp.Offset != 5 || resp.Limit == nil || *resp.Limit != 1 || resp.TotalCount != 7 {
		t.Errorf("resp = %+v", resp)
	}
	results, total, err := c.Search.Search(context.Background(), SearchParams{Query: "q"})
	if err != nil || len(results) != 1 || total != 7 {
		t.Errorf("Search wrapper: %v %d %v", results, total, err)
	}
}
