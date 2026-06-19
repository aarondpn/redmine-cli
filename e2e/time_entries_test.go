//go:build e2e

package e2e

import (
	"strconv"
	"testing"
)

// TestTimeEntries_CRUD covers log → list → update → delete. It validates
// hours parsing, activity name resolution, spent_on date handling, and that
// the list filter scoped to the issue returns just the logged entry.
func TestTimeEntries_CRUD(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)
	activity := firstActivityName(t, r)

	var created struct {
		ID       int     `json:"id"`
		Hours    float64 `json:"hours"`
		SpentOn  string  `json:"spent_on"`
		Comments string  `json:"comments"`
		Activity struct {
			Name string `json:"name"`
		} `json:"activity"`
	}
	r.runJSON(t, &created, "time", "log",
		"--issue", strconv.Itoa(issue.ID),
		"--hours", "1.25",
		"--activity", activity,
		"--date", "2026-04-17",
		"--comment", "logged by e2e")
	if created.Hours != 1.25 {
		t.Fatalf("time log hours = %v, want 1.25", created.Hours)
	}
	if created.SpentOn != "2026-04-17" {
		t.Fatalf("time log spent_on = %q, want 2026-04-17", created.SpentOn)
	}
	if created.Activity.Name == "" {
		t.Fatalf("time log activity not resolved: %+v", created)
	}

	var entries []struct {
		ID       int     `json:"id"`
		Hours    float64 `json:"hours"`
		Comments string  `json:"comments"`
	}
	r.runJSON(t, &entries, "time", "list", "--issue", strconv.Itoa(issue.ID))
	if len(entries) != 1 || entries[0].ID != created.ID {
		t.Fatalf("time list for issue %d = %+v, want single entry %d", issue.ID, entries, created.ID)
	}

	var updated actionEnvelope
	r.runJSON(t, &updated, "time", "update", strconv.Itoa(created.ID),
		"--hours", "2",
		"--comment", "edited by e2e")
	if !updated.Ok || updated.Action != "updated" || updated.Resource != "time_entry" {
		t.Fatalf("unexpected update envelope: %+v", updated)
	}

	var got struct {
		Hours    float64 `json:"hours"`
		Comments string  `json:"comments"`
	}
	r.runJSON(t, &got, "time", "get", strconv.Itoa(created.ID))
	if got.Hours != 2 {
		t.Fatalf("time get hours = %v after update, want 2", got.Hours)
	}
	if got.Comments != "edited by e2e" {
		t.Fatalf("time get comments = %q after update, want %q", got.Comments, "edited by e2e")
	}

	var deleted actionEnvelope
	r.runJSON(t, &deleted, "time", "delete", strconv.Itoa(created.ID), "--force")
	if !deleted.Ok || deleted.Action != "deleted" || deleted.Resource != "time_entry" {
		t.Fatalf("unexpected delete envelope: %+v", deleted)
	}

	r.runJSON(t, &entries, "time", "list", "--issue", strconv.Itoa(issue.ID))
	if len(entries) != 0 {
		t.Fatalf("expected no entries after delete; got %+v", entries)
	}
}

// TestTimeEntries_DefaultProjectIgnoredForIssueScope is a regression test for
// issue #123. When a default project is configured that differs from the
// issue's project, issue-scoped `time log` and `time list` must NOT inject the
// default project's project_id. Before the fix the injected project_id made
// Redmine reject the create (issue/project mismatch) and scoped the list query
// to the wrong project, so the entry was invisible.
func TestTimeEntries_DefaultProjectIgnoredForIssueScope(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	// projB owns the issue; projA is the configured default project.
	projA := createTestProject(t, r)
	projB := createTestProject(t, r)
	issue := createTestIssue(t, r, projB.Identifier)
	activity := firstActivityName(t, r)

	defaultProjectEnv := []string{"REDMINE_DEFAULT_PROJECT=" + projA.Identifier}

	// `time log --issue` must attach to the issue's project (projB), not the
	// configured default (projA).
	var created struct {
		ID      int `json:"id"`
		Project struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
	}
	r.runJSONWithEnv(t, defaultProjectEnv, &created, "time", "log",
		"--issue", strconv.Itoa(issue.ID),
		"--hours", "0.75",
		"--activity", activity,
		"--date", "2026-04-18",
		"--comment", "issue-scoped despite default project")
	if created.ID == 0 {
		t.Fatalf("time log returned no ID: %+v", created)
	}
	if created.Project.ID != projB.ID {
		t.Fatalf("time entry project = %d (%s), want issue's project %d (%s); default project leaked in",
			created.Project.ID, created.Project.Name, projB.ID, projB.Name)
	}

	t.Cleanup(func() {
		var deleted actionEnvelope
		r.runJSON(t, &deleted, "time", "delete", strconv.Itoa(created.ID), "--force")
		if !deleted.Ok {
			t.Errorf("time delete envelope not ok: %+v", deleted)
		}
	})

	// `time list --issue` must return the entry even though the configured
	// default project is a different project.
	var entries []struct {
		ID int `json:"id"`
	}
	r.runJSONWithEnv(t, defaultProjectEnv, &entries, "time", "list", "--issue", strconv.Itoa(issue.ID))
	if len(entries) != 1 || entries[0].ID != created.ID {
		t.Fatalf("time list --issue %d with default project %s = %+v, want single entry %d",
			issue.ID, projA.Identifier, entries, created.ID)
	}
}

// TestTimeEntries_LogOnBehalfOf verifies admin can log time for another user
// via `time log --user`. Resolves the target user by login and confirms the
// returned entry's user.id matches the fixture, not the API caller.
func TestTimeEntries_LogOnBehalfOf(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)
	activity := firstActivityName(t, r)
	target := createTestUser(t, r)

	// Redmine validates that the target user is a project member with the
	// "log_time" permission. Add the fresh fixture user via memberships create
	// using the first available role (Manager/Developer in stock builds both
	// include log_time).
	var membership struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &membership, "memberships", "create",
		"--project", proj.Identifier,
		"--user-id", strconv.Itoa(target.ID),
		"--role-ids", strconv.Itoa(firstRoleID(t, r)))
	if membership.ID == 0 {
		t.Fatalf("membership create returned no ID for user %d on project %s", target.ID, proj.Identifier)
	}

	var created struct {
		ID    int     `json:"id"`
		Hours float64 `json:"hours"`
		User  struct {
			ID int `json:"id"`
		} `json:"user"`
	}
	r.runJSON(t, &created, "time", "log",
		"--issue", strconv.Itoa(issue.ID),
		"--hours", "0.5",
		"--activity", activity,
		"--user", strconv.Itoa(target.ID),
		"--comment", "logged on behalf by e2e")
	if created.ID == 0 {
		t.Fatalf("time log on behalf returned no ID: %+v", created)
	}
	if created.User.ID != target.ID {
		t.Fatalf("time log on behalf user.id = %d, want %d", created.User.ID, target.ID)
	}

	t.Cleanup(func() {
		var deleted actionEnvelope
		r.runJSON(t, &deleted, "time", "delete", strconv.Itoa(created.ID), "--force")
		if !deleted.Ok {
			t.Errorf("time delete envelope not ok: %+v", deleted)
		}
	})
}
