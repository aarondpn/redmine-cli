//go:build e2e

package e2e

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type journalEntry struct {
	ID           int    `json:"id"`
	Notes        string `json:"notes"`
	PrivateNotes bool   `json:"private_notes"`
}

// listJournals returns the journals of an issue via the CLI.
func listJournals(t *testing.T, r *cliRunner, issueID int) []journalEntry {
	t.Helper()
	var got struct {
		Journals []journalEntry `json:"journals"`
	}
	r.runJSON(t, &got, "issues", "get", strconv.Itoa(issueID), "--include", "journals")
	return got.Journals
}

func findJournal(journals []journalEntry, id int) (journalEntry, bool) {
	for _, j := range journals {
		if j.ID == id {
			return j, true
		}
	}
	return journalEntry{}, false
}

// createIssueWithComment creates an issue with a single comment and returns
// the issue ID and the comment's journal.
func createIssueWithComment(t *testing.T, r *cliRunner, projectIdentifier, notes string) (int, journalEntry) {
	t.Helper()
	issue := createTestIssue(t, r, projectIdentifier)
	r.run(t, "issues", "comment", strconv.Itoa(issue.ID), "-m", notes)
	for _, j := range listJournals(t, r, issue.ID) {
		if j.Notes == notes {
			return issue.ID, j
		}
	}
	t.Fatalf("comment %q not found in journals of issue %d", notes, issue.ID)
	return 0, journalEntry{}
}

func TestIssueCommentEdit(t *testing.T) {
	requireE2E(t)
	skipBelowRedmine(t, 5, 0, "journal edit API")
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issueID, j := createIssueWithComment(t, r, proj.Identifier, "original comment")
	jid := strconv.Itoa(j.ID)

	var env actionEnvelope
	r.runJSON(t, &env, "issues", "comment", "edit", jid, "-m", "edited comment")
	if !env.Ok || env.Action != "updated" || env.Resource != "journal" || envelopeIntID(env.ID) != j.ID {
		t.Fatalf("edit envelope = %+v", env)
	}
	got, ok := findJournal(listJournals(t, r, issueID), j.ID)
	if !ok || got.Notes != "edited comment" || got.PrivateNotes {
		t.Fatalf("after edit = %+v (found=%v)", got, ok)
	}

	r.runJSON(t, &env, "issues", "comment", "edit", jid, "--private")
	got, _ = findJournal(listJournals(t, r, issueID), j.ID)
	if !got.PrivateNotes || got.Notes != "edited comment" {
		t.Fatalf("after --private = %+v", got)
	}

	r.runJSON(t, &env, "issues", "comment", "edit", jid, "--public")
	got, _ = findJournal(listJournals(t, r, issueID), j.ID)
	if got.PrivateNotes || got.Notes != "edited comment" {
		t.Fatalf("after --public = %+v", got)
	}

	path := filepath.Join(t.TempDir(), "note.txt")
	if err := os.WriteFile(path, []byte("comment from file"), 0o600); err != nil {
		t.Fatal(err)
	}
	r.runJSON(t, &env, "issues", "comment", "edit", jid, "--message-file", path)
	got, _ = findJournal(listJournals(t, r, issueID), j.ID)
	if got.Notes != "comment from file" {
		t.Fatalf("after --message-file = %+v", got)
	}

	// Validation errors happen before any request.
	r.runExpectJSONError(t, "issues", "comment", "edit", jid)
	r.runExpectJSONError(t, "issues", "comment", "edit", jid, "--message", "")
	r.runExpectJSONError(t, "issues", "comment", "edit", jid, "--private", "--public")
	r.runExpectJSONError(t, "issues", "comment", "edit", jid, "--clear", "-m", "x")

	// The table view shows the journal ID for discovery.
	out := string(r.run(t, "issues", "get", strconv.Itoa(issueID), "--journals", "--output", "table"))
	if !strings.Contains(out, "[journal "+jid+"]") {
		t.Errorf("issues get table output lacks [journal %s]:\n%s", jid, out)
	}
}

func TestIssueCommentEdit_Clear(t *testing.T) {
	requireE2E(t)
	skipBelowRedmine(t, 5, 0, "journal edit API")
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issueID, j := createIssueWithComment(t, r, proj.Identifier, "to be cleared")
	jid := strconv.Itoa(j.ID)

	// Declined confirmation (empty stdin) changes nothing.
	var env actionEnvelope
	r.runJSON(t, &env, "issues", "comment", "edit", jid, "--clear")
	if env.Ok || env.Action != "updated" || env.Resource != "journal" {
		t.Fatalf("cancelled envelope = %+v", env)
	}
	got, ok := findJournal(listJournals(t, r, issueID), j.ID)
	if !ok || got.Notes != "to be cleared" {
		t.Fatalf("after cancelled clear = %+v (found=%v)", got, ok)
	}

	r.runJSON(t, &env, "issues", "comment", "edit", jid, "--clear", "--force")
	if !env.Ok || env.Action != "updated" || envelopeIntID(env.ID) != j.ID {
		t.Fatalf("clear envelope = %+v", env)
	}
	got, ok = findJournal(listJournals(t, r, issueID), j.ID)
	// Redmine 5.x and 7.0 destroy a journal left without notes and details; the
	// 6.x lines keep the row but issues#show hides notes-less journals. Either
	// way the API no longer lists it.
	if ok {
		t.Fatalf("emptied journal should not be listed by the API, got %+v", got)
	}
}

func TestIssueCommentEdit_UnsupportedBefore50(t *testing.T) {
	requireE2E(t)
	if redmineAtLeast(5, 0) {
		t.Skip("journal edit API is available on this Redmine version")
	}
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	_, j := createIssueWithComment(t, r, proj.Identifier, "cannot edit")

	stderr := r.runExpectJSONError(t, "issues", "comment", "edit", strconv.Itoa(j.ID), "-m", "nope")
	assertErrorCode(t, stderr, "forbidden")
	if !strings.Contains(string(stderr), "5.0") {
		t.Errorf("error should hint at Redmine 5.0, got: %s", stderr)
	}
}

func TestIssueCommentEdit_UnknownJournal(t *testing.T) {
	requireE2E(t)
	skipBelowRedmine(t, 5, 0, "journal edit API")
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	assertErrorCode(t, r.runExpectJSONError(t, "issues", "comment", "edit", "99999999", "-m", "x"), "not_found")
}

func TestMCPStdio_UpdateJournal(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issueID, j := createIssueWithComment(t, r, proj.Identifier, "mcp original")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	session := r.startMCPStdio(t, ctx, "--enable-writes")

	out, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "update_journal",
		Arguments: map[string]any{"id": j.ID, "notes": "mcp edited", "private_notes": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !redmineAtLeast(5, 0) {
		if !out.IsError {
			t.Fatalf("update_journal should fail on Redmine < 5.0, got %s", structuredJSON(out))
		}
		return
	}
	if out.IsError {
		t.Fatalf("update_journal: %s", structuredJSON(out))
	}
	got, ok := findJournal(listJournals(t, r, issueID), j.ID)
	if !ok || got.Notes != "mcp edited" || !got.PrivateNotes {
		t.Fatalf("after update_journal = %+v (found=%v)", got, ok)
	}

	// The same journal is visible through get_issue includes=journals.
	out, err = session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_issue",
		Arguments: map[string]any{"id": issueID, "includes": []string{"journals"}},
	})
	if err != nil || out.IsError {
		t.Fatalf("get_issue: err=%v result=%s", err, structuredJSON(out))
	}
	if s := structuredJSON(out); !strings.Contains(s, "mcp edited") {
		t.Errorf("get_issue result lacks the edited notes: %s", s)
	}

	out, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "update_journal", Arguments: map[string]any{"id": j.ID}})
	if err != nil || !out.IsError {
		t.Fatalf("update_journal without fields should be a tool error: err=%v result=%s", err, structuredJSON(out))
	}
}
