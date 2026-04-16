//go:build e2e

package e2e

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestIssues_Lifecycle covers create → list → close → reopen.
func TestIssues_Lifecycle(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)

	var listed []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &listed, "issues", "list", "--project", proj.Identifier, "--limit", "100")
	if !containsIssueID(listed, issue.ID) {
		t.Fatalf("issues list did not include issue %d", issue.ID)
	}

	var closed actionEnvelope
	r.runJSON(t, &closed, "issues", "close", issueIDArg(issue.ID), "--note", "Closed by e2e")
	if !closed.Ok || closed.Action != "closed" || closed.Resource != "issue" || envelopeIntID(closed.ID) != issue.ID {
		t.Fatalf("unexpected close envelope: %+v", closed)
	}

	after := getIssueStatus(t, r, issue.ID)
	if !strings.Contains(strings.ToLower(after), "closed") {
		t.Fatalf("expected closed status after close, got %q", after)
	}

	var reopened actionEnvelope
	r.runJSON(t, &reopened, "issues", "reopen", issueIDArg(issue.ID), "--note", "Reopened by e2e")
	if !reopened.Ok || reopened.Action != "reopened" || reopened.Resource != "issue" || envelopeIntID(reopened.ID) != issue.ID {
		t.Fatalf("unexpected reopen envelope: %+v", reopened)
	}

	after = getIssueStatus(t, r, issue.ID)
	if strings.Contains(strings.ToLower(after), "closed") {
		t.Fatalf("expected reopened issue to have non-closed status, got %q", after)
	}
}

// TestIssues_UpdateAndComment verifies that `issues update` maps flag values
// onto the right Redmine fields, and that `issues comment` appends a journal
// note visible via `issues get --journals`.
func TestIssues_UpdateAndComment(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)

	newSubject := "Updated subject " + strconv.Itoa(issue.ID)
	var updated actionEnvelope
	r.runJSON(t, &updated, "issues", "update", issueIDArg(issue.ID),
		"--subject", newSubject,
		"--done-ratio", "50",
		"--description", "rewritten description")
	if !updated.Ok || updated.Action != "updated" || updated.Resource != "issue" {
		t.Fatalf("unexpected update envelope: %+v", updated)
	}

	var after struct {
		Subject     string `json:"subject"`
		Description string `json:"description"`
		DoneRatio   int    `json:"done_ratio"`
	}
	r.runJSON(t, &after, "issues", "get", issueIDArg(issue.ID))
	if after.Subject != newSubject {
		t.Fatalf("issue subject = %q, want %q", after.Subject, newSubject)
	}
	if after.DoneRatio != 50 {
		t.Fatalf("issue done_ratio = %d, want 50", after.DoneRatio)
	}
	if !strings.Contains(after.Description, "rewritten") {
		t.Fatalf("issue description not updated: %q", after.Description)
	}

	commentBody := "Comment added by e2e " + strconv.Itoa(issue.ID)
	var commented actionEnvelope
	r.runJSON(t, &commented, "issues", "comment", issueIDArg(issue.ID), "-m", commentBody)
	if !commented.Ok || commented.Action != "commented" || commented.Resource != "issue" {
		t.Fatalf("unexpected comment envelope: %+v", commented)
	}

	var withJournals struct {
		Journals []struct {
			Notes string `json:"notes"`
		} `json:"journals"`
	}
	r.runJSON(t, &withJournals, "issues", "get", issueIDArg(issue.ID), "--journals")
	found := false
	for _, j := range withJournals.Journals {
		if strings.Contains(j.Notes, commentBody) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("journal did not contain comment %q; got %+v", commentBody, withJournals.Journals)
	}
}

// TestIssues_Assign verifies `issues assign` resolves a user by login and
// sets assigned_to on the issue.
func TestIssues_Assign(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)

	var assigned actionEnvelope
	r.runJSON(t, &assigned, "issues", "assign", issueIDArg(issue.ID), "me")
	if !assigned.Ok || assigned.Action != "assigned" || assigned.Resource != "issue" {
		t.Fatalf("unexpected assign envelope: %+v", assigned)
	}

	var after struct {
		AssignedTo struct {
			Login string `json:"login"`
			Name  string `json:"name"`
		} `json:"assigned_to"`
	}
	r.runJSON(t, &after, "issues", "get", issueIDArg(issue.ID))
	if after.AssignedTo.Name == "" {
		t.Fatalf("expected issue to be assigned after `issues assign`, got %+v", after)
	}
}

// TestIssues_Attachment verifies the multipart upload path: create an issue
// with --attach, then fetch via the raw api with ?include=attachments (the
// typed Issue model doesn't surface attachments) and assert the file is
// present.
func TestIssues_Attachment(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	attachPath := filepath.Join(t.TempDir(), "hello.txt")
	payload := []byte("hello from redmine-cli e2e\n")
	if err := os.WriteFile(attachPath, payload, 0o600); err != nil {
		t.Fatalf("write attach file: %v", err)
	}

	tracker := firstTrackerName(t, r)
	var created struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &created, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", tracker,
		"--subject", "E2E issue with attachment",
		"--attach", attachPath)
	if created.ID == 0 {
		t.Fatal("issues create returned no ID")
	}

	var resp struct {
		Issue struct {
			Attachments []struct {
				Filename    string `json:"filename"`
				Filesize    int    `json:"filesize"`
				ContentType string `json:"content_type"`
			} `json:"attachments"`
		} `json:"issue"`
	}
	r.runJSON(t, &resp, "api", "/issues/"+issueIDArg(created.ID)+".json", "-f", "include=attachments")
	found := false
	for _, att := range resp.Issue.Attachments {
		if att.Filename == "hello.txt" && att.Filesize == len(payload) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("attachment not found on issue; got %+v", resp.Issue.Attachments)
	}
}

func containsIssueID(issues []struct {
	ID int `json:"id"`
}, id int) bool {
	for _, i := range issues {
		if i.ID == id {
			return true
		}
	}
	return false
}

func getIssueStatus(t *testing.T, r *cliRunner, id int) string {
	t.Helper()
	var got struct {
		Status struct {
			Name string `json:"name"`
		} `json:"status"`
	}
	r.runJSON(t, &got, "issues", "get", issueIDArg(id))
	return got.Status.Name
}
