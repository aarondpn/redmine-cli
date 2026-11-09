package ops

import (
	"context"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

// lastQuery returns the decoded query string of the last recorded call.
func lastQuery(t *testing.T, calls *[]opsCall) url.Values {
	t.Helper()
	if len(*calls) == 0 {
		t.Fatal("no request recorded")
	}
	u, err := url.Parse((*calls)[len(*calls)-1].uri)
	if err != nil {
		t.Fatal(err)
	}
	return u.Query()
}

func wantQuery(t *testing.T, q url.Values, key string, want ...string) {
	t.Helper()
	if !slices.Equal(q[key], want) {
		t.Errorf("%s = %q, want %q (query %s)", key, q[key], want, q.Encode())
	}
}

func TestProjectStatusFilter(t *testing.T) {
	for in, want := range map[string]string{
		"":       "",
		"active": "1",
		"Closed": "5",
		"all":    "1|5|9",
		"5":      "5",
		"1|5":    "1|5",
	} {
		got, err := ProjectStatusFilter(in)
		if err != nil || got != want {
			t.Errorf("ProjectStatusFilter(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for in, want := range map[string]string{
		"archived": "never lists archived projects",
		"open":     "use active, closed, all, or a numeric status code",
		"1|":       "use active, closed, all, or a numeric status code",
		"*":        "use active, closed, all, or a numeric status code",
	} {
		_, err := ProjectStatusFilter(in)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("ProjectStatusFilter(%q) error = %v, want it to contain %q", in, err, want)
		}
	}
}

func TestListProjects_StatusSortAndExtraParams(t *testing.T) {
	client, calls := opsServer(t, map[string]string{"GET /projects.json": `{"projects":[],"total_count":0}`})
	_, err := ListProjects(context.Background(), client, ListProjectsInput{
		Status:      "all",
		Sort:        "name:desc",
		ExtraParams: models.FilterParams{"f[]": {"name", "created_on"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	q := lastQuery(t, calls)
	wantQuery(t, q, "status", "1|5|9")
	wantQuery(t, q, "sort", "name:desc")
	wantQuery(t, q, "f[]", "name", "created_on")
}

func TestListProjects_InvalidStatusSendsNothing(t *testing.T) {
	client, calls := opsServer(t, nil)
	if _, err := ListProjects(context.Background(), client, ListProjectsInput{Status: "archived"}); err == nil {
		t.Fatal("expected an error for status archived")
	}
	if len(*calls) != 0 {
		t.Fatalf("requests sent for an invalid status: %+v", *calls)
	}
}

func TestListTimeEntries_SortQueryAndExtraParams(t *testing.T) {
	client, calls := opsServer(t, map[string]string{"GET /time_entries.json": `{"time_entries":[],"total_count":0}`})
	_, err := ListTimeEntries(context.Background(), client, ListTimeEntriesInput{
		QueryID:     7,
		Sort:        "hours:desc",
		ExtraParams: models.FilterParams{"cf_3": {"Onsite"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	q := lastQuery(t, calls)
	wantQuery(t, q, "query_id", "7")
	wantQuery(t, q, "sort", "hours:desc")
	wantQuery(t, q, "cf_3", "Onsite")
}

func TestListUsers_SortAndExtraParams(t *testing.T) {
	client, calls := opsServer(t, map[string]string{"GET /users.json": `{"users":[],"total_count":0}`})
	_, err := ListUsers(context.Background(), client, ListUsersInput{
		Status:      "all",
		Sort:        "login",
		ExtraParams: models.FilterParams{"op[status]": {"*"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	q := lastQuery(t, calls)
	wantQuery(t, q, "status", "")
	wantQuery(t, q, "sort", "login")
	wantQuery(t, q, "op[status]", "*")
}

func TestListGroups_Builtin(t *testing.T) {
	client, calls := opsServer(t, map[string]string{"GET /groups.json": `{"groups":[],"total_count":0}`})
	if _, err := ListGroups(context.Background(), client, ListGroupsInput{Builtin: true}); err != nil {
		t.Fatal(err)
	}
	wantQuery(t, lastQuery(t, calls), "builtin", "1")
}

func TestListIssues_ExtraParamsArray(t *testing.T) {
	client, calls := opsServer(t, map[string]string{"GET /issues.json": `{"issues":[],"total_count":0}`})
	_, err := ListIssues(context.Background(), client, ListIssuesInput{
		StatusID:    "open",
		ExtraParams: models.FilterParams{"status_id": {"*"}, "f[]": {"status_id", "subject"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	q := lastQuery(t, calls)
	wantQuery(t, q, "status_id", "*")
	wantQuery(t, q, "f[]", "status_id", "subject")
}
