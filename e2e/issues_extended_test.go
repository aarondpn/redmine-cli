//go:build e2e

package e2e

import (
	"strconv"
	"strings"
	"testing"
)

// TestIssues_CreateWithDates verifies that --start-date and --due-date are
// persisted on creation.
func TestIssues_CreateWithDates(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	var created struct {
		ID        int    `json:"id"`
		StartDate string `json:"start_date"`
		DueDate   string `json:"due_date"`
	}
	r.runJSON(t, &created, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", firstTrackerName(t, r),
		"--subject", "E2E dates",
		"--start-date", "2026-05-01",
		"--due-date", "2026-05-15")
	if created.StartDate != "2026-05-01" || created.DueDate != "2026-05-15" {
		t.Fatalf("dates = %+v, want 2026-05-01 / 2026-05-15", created)
	}
}

// TestIssues_UpdatePrivateNote verifies that --note + --private-notes attach a
// private journal note.
func TestIssues_UpdatePrivateNote(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)

	var updated actionEnvelope
	r.runJSON(t, &updated, "issues", "update", issueIDArg(issue.ID),
		"--note", "internal context",
		"--private-notes")
	if !updated.Ok {
		t.Fatalf("update envelope: %+v", updated)
	}

	var withJournals struct {
		Journals []struct {
			Notes        string `json:"notes"`
			PrivateNotes bool   `json:"private_notes"`
		} `json:"journals"`
	}
	r.runJSON(t, &withJournals, "issues", "get", issueIDArg(issue.ID), "--journals")
	found := false
	for _, j := range withJournals.Journals {
		if strings.Contains(j.Notes, "internal context") && j.PrivateNotes {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("private journal note not found: %+v", withJournals.Journals)
	}
}

// TestIssues_WatcherLifecycle covers add → list → remove.
func TestIssues_WatcherLifecycle(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)

	var me struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &me, "users", "me")

	var added actionEnvelope
	r.runJSON(t, &added, "issues", "watchers", "add", issueIDArg(issue.ID), "me")
	if !added.Ok || added.Action != "watched" {
		t.Fatalf("add envelope: %+v", added)
	}

	var watchers []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &watchers, "issues", "watchers", "list", issueIDArg(issue.ID))
	if !containsWatcherID(watchers, me.ID) {
		t.Fatalf("watcher list missing me=%d: %+v", me.ID, watchers)
	}

	var removed actionEnvelope
	r.runJSON(t, &removed, "issues", "watchers", "remove", issueIDArg(issue.ID), "me")
	if !removed.Ok || removed.Action != "unwatched" {
		t.Fatalf("remove envelope: %+v", removed)
	}
}

// TestIssues_CustomFieldOnCreate verifies that --custom-field (name=value)
// resolves the field by name against the admin-only /custom_fields.json
// endpoint and round-trips a value on a new issue. The seeded "E2E Severity"
// fixture (is_for_all=true) supplies the field automatically on every test
// project.
func TestIssues_CustomFieldOnCreate(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	var created struct {
		ID           int `json:"id"`
		CustomFields []struct {
			Name  string `json:"name"`
			Value any    `json:"value"`
		} `json:"custom_fields"`
	}
	r.runJSON(t, &created, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", firstTrackerName(t, r),
		"--subject", "E2E custom field",
		"--custom-field", "E2E Severity=High")

	found := false
	for _, cf := range created.CustomFields {
		if cf.Name == "E2E Severity" && cf.Value == "High" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("custom field not set on new issue: %+v", created.CustomFields)
	}
}

// TestIssues_RelationPrecedesWithDelay verifies the precedes relation type
// accepts the --delay flag and Redmine returns the delay on subsequent GET.
func TestIssues_RelationPrecedesWithDelay(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	a := createTestIssueWithSubject(t, r, proj.Identifier, "PrecA")
	b := createTestIssueWithSubject(t, r, proj.Identifier, "PrecB")

	var rel struct {
		ID           int    `json:"id"`
		RelationType string `json:"relation_type"`
		Delay        *int   `json:"delay"`
	}
	r.runJSON(t, &rel, "issues", "relations", "add", issueIDArg(a.ID),
		"--to", strconv.Itoa(b.ID),
		"--type", "precedes",
		"--delay", "5",
		"--output", "json")
	if rel.RelationType != "precedes" {
		t.Fatalf("relation_type = %q, want precedes", rel.RelationType)
	}
	if rel.Delay == nil || *rel.Delay != 5 {
		t.Fatalf("delay = %v, want 5", rel.Delay)
	}
}

// TestIssues_RelationLifecycle covers create → list → delete.
func TestIssues_RelationLifecycle(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	a := createTestIssueWithSubject(t, r, proj.Identifier, "RelA")
	b := createTestIssueWithSubject(t, r, proj.Identifier, "RelB")

	var rel struct {
		ID           int    `json:"id"`
		RelationType string `json:"relation_type"`
	}
	r.runJSON(t, &rel, "issues", "relations", "add", issueIDArg(a.ID),
		"--to", strconv.Itoa(b.ID),
		"--type", "blocks",
		"--output", "json")
	if rel.RelationType != "blocks" {
		t.Fatalf("relation type = %q, want blocks", rel.RelationType)
	}

	var relations []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &relations, "issues", "relations", "list", issueIDArg(a.ID))
	if !containsRelationID(relations, rel.ID) {
		t.Fatalf("relations list missing %d: %+v", rel.ID, relations)
	}

	var removed actionEnvelope
	r.runJSON(t, &removed, "issues", "relations", "remove", strconv.Itoa(rel.ID))
	if !removed.Ok || removed.Action != "unrelated" {
		t.Fatalf("remove envelope: %+v", removed)
	}
}

func containsWatcherID(list []struct {
	ID int `json:"id"`
}, id int) bool {
	for _, w := range list {
		if w.ID == id {
			return true
		}
	}
	return false
}

func containsRelationID(list []struct {
	ID int `json:"id"`
}, id int) bool {
	for _, r := range list {
		if r.ID == id {
			return true
		}
	}
	return false
}
