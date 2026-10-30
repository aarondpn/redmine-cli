//go:build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

type enumItem struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	Active    bool   `json:"active"`
}

type rawEnumResponse struct {
	IssuePriorities     []enumItem `json:"issue_priorities"`
	TimeEntryActivities []enumItem `json:"time_entry_activities"`
	DocumentCategories  []enumItem `json:"document_categories"`
}

func enumIDs(items []enumItem) []int {
	ids := make([]int, len(items))
	for i, it := range items {
		ids[i] = it.ID
	}
	return ids
}

func sameIDs(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestEnumerations_PrioritiesList(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var got []enumItem
	r.runJSON(t, &got, "priorities", "list")
	if len(got) == 0 {
		t.Fatal("priorities list returned no priorities")
	}
	defaults := 0
	for _, p := range got {
		if p.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Errorf("want exactly one default priority, got %d: %+v", defaults, got)
	}

	var raw rawEnumResponse
	r.runJSON(t, &raw, "api", "/enumerations/issue_priorities.json")
	if !sameIDs(enumIDs(got), enumIDs(raw.IssuePriorities)) {
		t.Errorf("CLI priority IDs %v differ from raw %v", enumIDs(got), enumIDs(raw.IssuePriorities))
	}

	// The IDs/names are usable: creating an issue by priority name lands on the listed ID.
	target := got[len(got)-1]
	proj := createTestProject(t, r)
	var created struct {
		Priority struct {
			ID int `json:"id"`
		} `json:"priority"`
	}
	r.runJSON(t, &created, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", firstTrackerName(t, r),
		"--subject", "Priority round trip",
		"--priority", target.Name)
	if created.Priority.ID != target.ID {
		t.Errorf("issue priority id = %d, want %d (%s)", created.Priority.ID, target.ID, target.Name)
	}

	// Alias and table output.
	out := r.run(t, "priority", "ls", "-o", "table")
	if !strings.Contains(string(out), target.Name) {
		t.Errorf("table output missing %q:\n%s", target.Name, out)
	}
}

func TestEnumerations_ActivitiesList(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var global []enumItem
	r.runJSON(t, &global, "activities", "list")
	if len(global) < 2 {
		t.Fatalf("expected at least two shared activities, got %+v", global)
	}
	var raw rawEnumResponse
	r.runJSON(t, &raw, "api", "/enumerations/time_entry_activities.json")
	if !sameIDs(enumIDs(global), enumIDs(raw.TimeEntryActivities)) {
		t.Errorf("CLI activity IDs %v differ from raw %v", enumIDs(global), enumIDs(raw.TimeEntryActivities))
	}

	var project []struct {
		ID     int    `json:"id"`
		Name   string `json:"name"`
		Active bool   `json:"active"`
	}
	r.runJSON(t, &project, "activities", "list", "--project", contractProject)
	if len(project) == 0 {
		t.Fatal("activities list --project returned nothing")
	}

	globalByID := map[int]string{}
	globalByName := map[string]int{}
	for _, a := range global {
		globalByID[a.ID] = a.Name
		globalByName[a.Name] = a.ID
	}
	overrides := 0
	for _, a := range project {
		if !a.Active {
			t.Errorf("project activity %+v must be reported active", a)
		}
		sharedID, sameName := globalByName[a.Name]
		if _, isShared := globalByID[a.ID]; !isShared && sameName {
			overrides++
			if sharedID == a.ID {
				t.Errorf("override %q reuses the shared ID %d", a.Name, a.ID)
			}
		}
	}
	if overrides == 0 {
		t.Errorf("project list %+v shows no override (same name as a shared activity, different ID); global: %+v", project, global)
	}

	out := r.run(t, "activity", "ls", "--project", contractProject, "-o", "table")
	if strings.Contains(string(out), "Default") {
		t.Errorf("project table must only show ID and Name:\n%s", out)
	}
}

func TestEnumerations_DocumentCategoriesList(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var got []enumItem
	r.runJSON(t, &got, "document-categories", "list")
	if len(got) == 0 {
		t.Fatal("document-categories list returned nothing")
	}
	var raw rawEnumResponse
	r.runJSON(t, &raw, "api", "/enumerations/document_categories.json")
	if !sameIDs(enumIDs(got), enumIDs(raw.DocumentCategories)) {
		t.Errorf("CLI IDs %v differ from raw %v", enumIDs(got), enumIDs(raw.DocumentCategories))
	}
	out := r.run(t, "document-category", "ls", "-o", "table")
	if !strings.Contains(string(out), got[0].Name) {
		t.Errorf("table output missing %q:\n%s", got[0].Name, out)
	}
}

func TestEnumerations_ActivitiesUnknownProject(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	stderr := r.runExpectJSONError(t, "activities", "list", "--project", "no-such-project-e2e-204")
	// The project resolver reports an unmatched name with a plain message, so
	// the envelope code is the generic one; the message must name the project.
	_ = requireErrorEnvelopeMessage(t, stderr)
	if !strings.Contains(string(stderr), "no-such-project-e2e-204") {
		t.Errorf("error should mention the project:\n%s", stderr)
	}
}

func TestMCPStdio_EnumerationTools(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	session := r.startMCPStdio(t, ctx)

	idsFrom := func(t *testing.T, v any) []int {
		t.Helper()
		items, _ := v.([]any)
		var ids []int
		for _, it := range items {
			ids = append(ids, jsonInt(field(it, "id")))
		}
		return ids
	}

	t.Run("priorities", func(t *testing.T) {
		var cli []enumItem
		r.runJSON(t, &cli, "priorities", "list")
		res := mcpCallAny(t, ctx, session, "list_priorities", map[string]any{})
		got := idsFrom(t, field(res, "issue_priorities"))
		if !sameIDs(got, enumIDs(cli)) {
			t.Errorf("MCP priority IDs %v != CLI %v", got, enumIDs(cli))
		}
		if jsonInt(field(res, "count")) != len(cli) {
			t.Errorf("count = %v, want %d", field(res, "count"), len(cli))
		}
	})

	t.Run("activities global", func(t *testing.T) {
		var cli []enumItem
		r.runJSON(t, &cli, "activities", "list")
		res := mcpCallAny(t, ctx, session, "list_time_entry_activities", map[string]any{})
		got := idsFrom(t, field(res, "time_entry_activities"))
		if !sameIDs(got, enumIDs(cli)) {
			t.Errorf("MCP activity IDs %v != CLI %v", got, enumIDs(cli))
		}
	})

	t.Run("activities project", func(t *testing.T) {
		var cli []enumItem
		r.runJSON(t, &cli, "activities", "list", "--project", contractProject)
		res := mcpCallAny(t, ctx, session, "list_time_entry_activities", map[string]any{"project_id": contractProject})
		got := idsFrom(t, field(res, "time_entry_activities"))
		if !sameIDs(got, enumIDs(cli)) {
			t.Errorf("MCP project activity IDs %v != CLI %v", got, enumIDs(cli))
		}
	})

	t.Run("groups", func(t *testing.T) {
		timeOnly := r.startMCPStdio(t, ctx, "--enable-groups", "time")
		tools, err := timeOnly.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("ListTools: %v", err)
		}
		mustContain(t, toolNames(tools.Tools), "list_time_entry_activities")

		metaOnly := r.startMCPStdio(t, ctx, "--enable-groups", "meta")
		tools, err = metaOnly.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("ListTools: %v", err)
		}
		mustContain(t, toolNames(tools.Tools), "list_priorities")
	})
}

func TestRelations_Get(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	a := createTestIssueWithSubject(t, r, proj.Identifier, "Relation get source")
	b := createTestIssueWithSubject(t, r, proj.Identifier, "Relation get target")

	var rel struct {
		ID           int    `json:"id"`
		IssueID      int    `json:"issue_id"`
		IssueToID    int    `json:"issue_to_id"`
		RelationType string `json:"relation_type"`
	}
	r.runJSON(t, &rel, "issues", "relations", "add", issueIDArg(a.ID), "--to", issueIDArg(b.ID), "--type", "blocks")
	if rel.ID == 0 {
		t.Fatal("relation add returned no ID")
	}

	var got struct {
		ID           int    `json:"id"`
		IssueID      int    `json:"issue_id"`
		IssueToID    int    `json:"issue_to_id"`
		RelationType string `json:"relation_type"`
	}
	r.runJSON(t, &got, "issues", "relations", "get", fmt.Sprint(rel.ID))
	if got.ID != rel.ID || got.IssueID != a.ID || got.IssueToID != b.ID || got.RelationType != "blocks" {
		t.Errorf("relation get = %+v, want id=%d %d blocks %d", got, rel.ID, a.ID, b.ID)
	}

	out := r.run(t, "issues", "relations", "view", fmt.Sprint(rel.ID), "-o", "table")
	for _, want := range []string{fmt.Sprintf("#%d", a.ID), fmt.Sprintf("#%d", b.ID), "blocks"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}

	stderr := r.runExpectJSONError(t, "issues", "relations", "get", "999999999")
	assertErrorCode(t, stderr, "not_found")
}
