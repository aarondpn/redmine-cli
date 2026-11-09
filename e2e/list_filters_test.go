//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// timeTestQuery is the time entry query seeded by bootstrap-redmine.sh: it
// keeps entries of 2 hours or more.
const timeTestQuery = "E2E Long Time Entries"

type listedTimeEntry struct {
	ID    int     `json:"id"`
	Hours float64 `json:"hours"`
}

func timeEntryIDs(entries []listedTimeEntry) []int {
	ids := make([]int, len(entries))
	for i, e := range entries {
		ids[i] = e.ID
	}
	return ids
}

// logTestTime logs hours on the issue for today and returns the entry ID.
func logTestTime(t *testing.T, r *cliRunner, issueID int, hours, activity string) int {
	t.Helper()
	var created struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &created, "time", "log", "--issue", strconv.Itoa(issueID), "--hours", hours,
		"--activity", activity, "--comment", "list filters e2e")
	if created.ID == 0 {
		t.Fatalf("time log returned no ID")
	}
	return created.ID
}

// TestListFilters_TimeEntries covers --sort, --filter (plain keys, operator
// values, f[]/op[]/v[]) and --query / --query-id on `time list`.
func TestListFilters_TimeEntries(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)
	activity := firstActivityName(t, r)
	short := logTestTime(t, r, issue.ID, "1", activity)
	long := logTestTime(t, r, issue.ID, "3", activity)

	list := func(args ...string) []int {
		t.Helper()
		var entries []listedTimeEntry
		r.runJSON(t, &entries, append([]string{"time", "list", "--project", proj.Identifier}, args...)...)
		return timeEntryIDs(entries)
	}
	today := time.Now().Format("2006-01-02")

	cases := []struct {
		name string
		args []string
		want []int
	}{
		{"sort hours asc", []string{"--sort", "hours"}, []int{short, long}},
		{"sort hours desc", []string{"--sort", "hours:desc"}, []int{long, short}},
		{"plain operator filter", []string{"--filter", "hours=>=2"}, []int{long}},
		{"date range filter", []string{"--filter", "spent_on=><2000-01-01|2000-01-31"}, []int{}},
		{"f[] op[] v[]", []string{
			"--filter", "f[]=hours", "--filter", "op[hours]=<=", "--filter", "v[hours][]=2",
			"--filter", "f[]=spent_on", "--filter", "op[spent_on]=><",
			"--filter", "v[spent_on][]=2000-01-01", "--filter", "v[spent_on][]=" + today,
		}, []int{short}},
		{"saved query by name", []string{"--query", timeTestQuery}, []int{long}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := list(tc.args...)
			if tc.name == "sort hours asc" || tc.name == "sort hours desc" {
				if !slices.Equal(got, tc.want) {
					t.Fatalf("time list %v = %v, want %v in this order", tc.args, got, tc.want)
				}
				return
			}
			slices.Sort(got)
			want := slices.Clone(tc.want)
			slices.Sort(want)
			if !slices.Equal(got, want) {
				t.Fatalf("time list %v = %v, want %v", tc.args, got, want)
			}
		})
	}

	t.Run("saved query by id", func(t *testing.T) {
		var queries []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		var resp struct {
			Queries []struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			} `json:"queries"`
		}
		r.runJSON(t, &resp, "api", "/queries.json?type=TimeEntryQuery&limit=100")
		queries = resp.Queries
		id := 0
		for _, q := range queries {
			if q.Name == timeTestQuery {
				id = q.ID
			}
		}
		if id == 0 {
			t.Fatalf("seeded time entry query %q not found in %+v", timeTestQuery, queries)
		}
		if got := list("--query-id", strconv.Itoa(id)); !slices.Equal(got, []int{long}) {
			t.Fatalf("time list --query-id %d = %v, want [%d]", id, got, long)
		}
	})

	t.Run("mcp", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		session := r.startMCPStdio(t, ctx)
		out := mcpCallOK(t, ctx, session, "list_time_entries", map[string]any{
			"project_id": proj.Identifier,
			"sort":       "hours:desc",
			"extra_params": map[string]any{
				"f[]":        []string{"hours"},
				"op[hours]":  ">=",
				"v[hours][]": []string{"0.5"},
			},
		})
		var result struct {
			TimeEntries []listedTimeEntry `json:"time_entries"`
		}
		decodeStructured(t, out, &result)
		if got := timeEntryIDs(result.TimeEntries); !slices.Equal(got, []int{long, short}) {
			t.Fatalf("list_time_entries = %v, want [%d %d]", got, long, short)
		}
	})
}

// TestListFilters_IssueRepeatedKeys checks that keys ending in [] send every
// value: with f[] Redmine replaces its default open-status filter, so a closed
// issue matched by subject is listed.
func TestListFilters_IssueRepeatedKeys(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	marker := "rk" + uniqueShortSuffix(t)
	open := createTestIssueWithSubject(t, r, proj.Identifier, marker+" open")
	closed := createTestIssueWithSubject(t, r, proj.Identifier, marker+" closed")
	r.run(t, "issues", "close", strconv.Itoa(closed.ID))

	var issues []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &issues, "issues", "list", "--project", proj.Identifier,
		"--filter", "f[]=status_id", "--filter", "op[status_id]=*",
		"--filter", "f[]=subject", "--filter", "op[subject]=~", "--filter", "v[subject][]="+marker)
	var ids []int
	for _, i := range issues {
		ids = append(ids, i.ID)
	}
	slices.Sort(ids)
	want := []int{open.ID, closed.ID}
	slices.Sort(want)
	if !slices.Equal(ids, want) {
		t.Fatalf("issues list with f[] filters = %v, want %v", ids, want)
	}
}

// closeTestProject closes a project through the raw API (Redmine 5.0+) and
// reopens it on cleanup so the fixture delete runs against an active project.
func closeTestProject(t *testing.T, r *cliRunner, identifier string) {
	t.Helper()
	empty := filepath.Join(t.TempDir(), "empty.json")
	if err := os.WriteFile(empty, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	r.run(t, "api", "/projects/"+identifier+"/close.json", "-X", "PUT", "--input", empty)
	t.Cleanup(func() {
		_, _, _ = r.runRaw("api", "/projects/"+identifier+"/reopen.json", "-X", "PUT", "--input", empty)
	})
}

func listProjectIDs(t *testing.T, r *cliRunner, args ...string) []int {
	t.Helper()
	var projects []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &projects, append([]string{"projects", "list", "--limit", "0"}, args...)...)
	ids := make([]int, len(projects))
	for i, p := range projects {
		ids[i] = p.ID
	}
	return ids
}

// TestListFilters_ProjectStatus covers --status on `projects list`: Redmine
// hides closed projects unless asked.
func TestListFilters_ProjectStatus(t *testing.T) {
	requireE2E(t)
	skipBelowRedmine(t, 5, 0, "closing projects over REST")
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	active := createTestProject(t, r)
	closed := createTestProject(t, r)
	closeTestProject(t, r, closed.Identifier)

	cases := []struct {
		args              []string
		active, closedHit bool
	}{
		{nil, true, false},
		{[]string{"--status", "active"}, true, false},
		{[]string{"--status", "closed"}, false, true},
		{[]string{"--status", "all"}, true, true},
		{[]string{"--status", "5"}, false, true},
	}
	for _, tc := range cases {
		ids := listProjectIDs(t, r, tc.args...)
		if containsInt(ids, active.ID) != tc.active || containsInt(ids, closed.ID) != tc.closedHit {
			t.Errorf("projects list %v: active listed=%v (want %v), closed listed=%v (want %v)",
				tc.args, containsInt(ids, active.ID), tc.active, containsInt(ids, closed.ID), tc.closedHit)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	session := r.startMCPStdio(t, ctx)
	out := mcpCallOK(t, ctx, session, "list_projects", map[string]any{"status": "closed", "limit": 100})
	var result struct {
		Projects []struct {
			ID int `json:"id"`
		} `json:"projects"`
	}
	decodeStructured(t, out, &result)
	var ids []int
	for _, p := range result.Projects {
		ids = append(ids, p.ID)
	}
	if !containsInt(ids, closed.ID) || containsInt(ids, active.ID) {
		t.Errorf("list_projects status=closed = %v, want closed %d without active %d", ids, closed.ID, active.ID)
	}
}

// TestListFilters_ProjectSortAndFilter covers --sort and --filter on
// `projects list` (ProjectQuery, every supported version).
func TestListFilters_ProjectSortAndFilter(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	a := createTestProject(t, r)
	b := createTestProject(t, r)
	// Both names are "CLI E2E e2e-<test name>-<UnixNano>" and differ only in
	// the equally long suffix, so name order equals creation order.
	marker := "~CLI E2E e2e-" + sanitizeForIdentifier(t.Name()) + "-"

	asc := listProjectIDs(t, r, "--filter", "name="+marker, "--sort", "name")
	desc := listProjectIDs(t, r, "--filter", "name="+marker, "--sort", "name:desc")
	if !slices.Equal(asc, []int{a.ID, b.ID}) || !slices.Equal(desc, []int{b.ID, a.ID}) {
		t.Fatalf("filter/sort not applied: asc=%v desc=%v (a=%d b=%d)", asc, desc, a.ID, b.ID)
	}
	if got := listProjectIDs(t, r, "--filter", "name=~no-such-project-"+uniqueShortSuffix(t)); len(got) != 0 {
		t.Fatalf("name filter matched nothing but listed %v", got)
	}
}

// TestListFilters_UserStatusAll covers `users list --status all`, which must
// list a locked user on every supported version (empty status= on 4.2-5.1,
// the status=* retry on 6.0+).
func TestListFilters_UserStatusAll(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	u := createTestUser(t, r)
	lockUser(t, r, u.ID)

	if ids := listUserIDs(t, r, "--name", u.Login, "--status", "all"); !containsInt(ids, u.ID) {
		t.Fatalf("users list --status all --name %s = %v, want locked user %d", u.Login, ids, u.ID)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	session := r.startMCPStdio(t, ctx)
	out := mcpCallOK(t, ctx, session, "list_users", map[string]any{"status": "all", "name": u.Login})
	var result struct {
		Users []struct {
			ID int `json:"id"`
		} `json:"users"`
	}
	decodeStructured(t, out, &result)
	if len(result.Users) != 1 || result.Users[0].ID != u.ID {
		t.Fatalf("list_users status=all = %+v, want user %d", result.Users, u.ID)
	}
}

// TestListFilters_UserSortAndQueryFilters covers --sort (every version) and
// the generic f[]/op[]/v[] filters (UserQuery, gated in gatedListParams).
func TestListFilters_UserSortAndQueryFilters(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	// The last names carry the order; --name matches them too.
	marker := "lf" + uniqueShortSuffix(t)
	a := createTestUser(t, r, "--lastname", marker+"a")
	b := createTestUser(t, r, "--lastname", marker+"b")

	asc := listUserIDs(t, r, "--name", marker, "--sort", "lastname")
	desc := listUserIDs(t, r, "--name", marker, "--sort", "lastname:desc")
	if !slices.Equal(asc, []int{a.ID, b.ID}) || !slices.Equal(desc, []int{b.ID, a.ID}) {
		t.Fatalf("sort not applied: asc=%v desc=%v (a=%d b=%d)", asc, desc, a.ID, b.ID)
	}

	t.Run("query filters", func(t *testing.T) {
		skipUnlessListParam(t, "users_query_filters")
		lockUser(t, r, b.ID)
		ids := listUserIDs(t, r,
			"--filter", "f[]=status", "--filter", "op[status]=*",
			"--filter", "f[]=lastname", "--filter", "op[lastname]=~", "--filter", "v[lastname][]="+marker,
			"--sort", "lastname")
		if !slices.Equal(ids, []int{a.ID, b.ID}) {
			t.Fatalf("users list with f[] filters = %v, want [%d %d] (active and locked)", ids, a.ID, b.ID)
		}
	})
}

// TestListFilters_GroupsBuiltin covers `groups list --builtin`.
func TestListFilters_GroupsBuiltin(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	type group struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	var plain, builtin []group
	r.runJSON(t, &plain, "groups", "list")
	r.runJSON(t, &builtin, "groups", "list", "--builtin")
	if len(builtin) != len(plain)+2 {
		t.Fatalf("groups list --builtin = %d groups, want the %d givable ones plus Anonymous and Non member", len(builtin), len(plain))
	}
	for _, g := range plain {
		if !slices.ContainsFunc(builtin, func(b group) bool { return b.ID == g.ID }) {
			t.Errorf("group %d missing from --builtin list", g.ID)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	session := r.startMCPStdio(t, ctx)
	out := mcpCallOK(t, ctx, session, "list_groups", map[string]any{"builtin": true, "limit": 100})
	var result struct {
		Groups []group `json:"groups"`
	}
	decodeStructured(t, out, &result)
	if len(result.Groups) != len(builtin) {
		t.Fatalf("list_groups builtin=true = %d groups, want %d", len(result.Groups), len(builtin))
	}
}

// decodeStructured decodes an MCP tool result into dest, from the structured
// content when present and the text content otherwise.
func decodeStructured(t *testing.T, out *mcp.CallToolResult, dest any) {
	t.Helper()
	var data []byte
	if out.StructuredContent != nil {
		var err error
		if data, err = json.Marshal(out.StructuredContent); err != nil {
			t.Fatalf("marshal structured content: %v", err)
		}
	} else {
		data = []byte(mcpExtTextContent(t, out))
	}
	if err := json.Unmarshal(data, dest); err != nil {
		t.Fatalf("decode structured content: %v\n%s", err, data)
	}
}
