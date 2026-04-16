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
