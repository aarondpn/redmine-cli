//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// issueState is the part of `issues get -o json` the write-support tests
// assert on.
type issueState struct {
	ID      int    `json:"id"`
	Subject string `json:"subject"`
	Project struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"project"`
	DoneRatio  int `json:"done_ratio"`
	AssignedTo *struct {
		ID int `json:"id"`
	} `json:"assigned_to"`
	Attachments []attachmentEntry `json:"attachments"`
	Children    []struct {
		ID      int    `json:"id"`
		Subject string `json:"subject"`
	} `json:"children"`
	Relations []struct {
		IssueID      int    `json:"issue_id"`
		IssueToID    int    `json:"issue_to_id"`
		RelationType string `json:"relation_type"`
	} `json:"relations"`
	Watchers []struct {
		ID int `json:"id"`
	} `json:"watchers"`
}

// errorEnvelopeWithCode asserts the error code and returns the envelope.
func errorEnvelopeWithCode(t *testing.T, stderr []byte, code string) errorEnvelope {
	t.Helper()
	assertErrorCode(t, stderr, code)
	var env errorEnvelope
	if err := json.Unmarshal(lastLine(stderr), &env); err != nil {
		t.Fatal(err)
	}
	return env
}

func getIssueState(t *testing.T, r *cliRunner, id int) issueState {
	t.Helper()
	var got issueState
	r.runJSON(t, &got, "issues", "get", issueIDArg(id), "--include", "attachments,children,relations,watchers")
	return got
}

func writeAttachFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestIssueUpdate_MoveToProject(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	source := createTestProject(t, r)
	target := createTestProject(t, r)
	issue := createTestIssue(t, r, source.Identifier)

	// By name, like --project elsewhere.
	var env actionEnvelope
	r.runJSON(t, &env, "issues", "update", issueIDArg(issue.ID), "--project", target.Name)
	if !env.Ok || env.Action != "updated" {
		t.Fatalf("update envelope = %+v", env)
	}
	if got := getIssueState(t, r, issue.ID); got.Project.ID != target.ID {
		t.Fatalf("project after move = %+v, want %d", got.Project, target.ID)
	}
}

// Redmine drops a move to a project without the issue tracking module and
// still answers 204; the read-back turns that into project_not_applied.
func TestIssueUpdate_MoveIgnoredIsReported(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	source := createTestProject(t, r)
	target := createTestProject(t, r)
	r.run(t, "projects", "update", target.Identifier, "--enable-module", "wiki")
	issue := createTestIssue(t, r, source.Identifier)

	stderr := r.runExpectJSONError(t, "issues", "update", issueIDArg(issue.ID), "--project", target.Identifier, "--subject", "Renamed")
	env := errorEnvelopeWithCode(t, stderr, "project_not_applied")
	if len(env.Error.Details) == 0 || !strings.Contains(env.Error.Details[0], "may already have been applied") {
		t.Fatalf("details = %v", env.Error.Details)
	}
	got := getIssueState(t, r, issue.ID)
	if got.Project.ID != source.ID || got.Subject != "Renamed" {
		t.Fatalf("issue after ignored move = %+v", got)
	}
}

func TestIssueUpdate_RemoveAttachment(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issueID, filename, _ := createIssueWithAttachment(t, r, proj.Identifier)
	att := findIssueAttachmentID(t, r, issueID, filename)

	// Declined (no input) keeps the attachment.
	var env actionEnvelope
	r.runJSON(t, &env, "issues", "update", issueIDArg(issueID), "--remove-attachment", strconv.Itoa(att.ID))
	if env.Ok {
		t.Fatalf("declined envelope = %+v", env)
	}
	if got := getIssueState(t, r, issueID); len(got.Attachments) != 1 {
		t.Fatalf("attachments after decline = %+v", got.Attachments)
	}

	r.runJSON(t, &env, "issues", "update", issueIDArg(issueID), "--remove-attachment", strconv.Itoa(att.ID), "--force")
	if !env.Ok {
		t.Fatalf("remove envelope = %+v", env)
	}
	if got := getIssueState(t, r, issueID); len(got.Attachments) != 0 {
		t.Fatalf("attachments after removal = %+v", got.Attachments)
	}
	assertErrorCode(t, r.runExpectJSONError(t, "attachments", "get", strconv.Itoa(att.ID)), "not_found")
}

func TestIssueCreate_DoneRatio(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	var created issueState
	r.runJSON(t, &created, "issues", "create", "--project", proj.Identifier, "--tracker", firstTrackerName(t, r),
		"--subject", "E2E done ratio", "--done-ratio", "40")
	if created.DoneRatio != 40 {
		t.Fatalf("done_ratio = %d, want 40", created.DoneRatio)
	}
}

// --attach-description pairs with --attach by position on issues, wiki and
// news alike.
func TestAttachDescriptions(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	first := writeAttachFile(t, "first-"+suffix+".txt", "one")
	second := writeAttachFile(t, "second-"+suffix+".txt", "two")

	var created issueState
	r.runJSON(t, &created, "issues", "create", "--project", proj.Identifier, "--tracker", firstTrackerName(t, r),
		"--subject", "E2E attach descriptions",
		"--attach", first, "--attach-description", "First file",
		"--attach", second)
	descriptions := map[string]string{}
	for _, a := range getIssueState(t, r, created.ID).Attachments {
		descriptions[a.Filename] = a.Description
	}
	if descriptions[filepath.Base(first)] != "First file" || descriptions[filepath.Base(second)] != "" {
		t.Fatalf("issue attachment descriptions = %v", descriptions)
	}

	third := writeAttachFile(t, "third-"+suffix+".txt", "three")
	r.run(t, "issues", "update", issueIDArg(created.ID), "--attach", third, "--attach-description", "Third file")
	found := false
	for _, a := range getIssueState(t, r, created.ID).Attachments {
		if a.Filename == filepath.Base(third) {
			found = a.Description == "Third file"
		}
	}
	if !found {
		t.Fatal("update --attach-description did not reach the attachment")
	}

	// More descriptions than files fails before anything is sent.
	stderr := r.runExpectJSONError(t, "issues", "update", issueIDArg(created.ID), "--subject", "Nope", "--attach-description", "orphan")
	if !strings.Contains(string(stderr), "--attach-description given 1 time(s)") {
		t.Fatalf("stderr = %s", stderr)
	}
	if got := getIssueState(t, r, created.ID); got.Subject == "Nope" {
		t.Fatal("the update was sent despite the flag error")
	}

	page := "AttachDesc" + suffix
	r.run(t, "wiki", "create", page, "--project", proj.Identifier, "--text", "h1. Files",
		"--attach", writeAttachFile(t, "wiki-"+suffix+".txt", "w"), "--attach-description", "Wiki file")
	var wiki struct {
		Attachments []attachmentEntry `json:"attachments"`
	}
	r.runJSON(t, &wiki, "wiki", "get", page, "--project", proj.Identifier, "--include", "attachments")
	if len(wiki.Attachments) != 1 || wiki.Attachments[0].Description != "Wiki file" {
		t.Fatalf("wiki attachments = %+v", wiki.Attachments)
	}
}

func TestIssueCopy(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	source := createTestProject(t, r)
	target := createTestProject(t, r)
	tracker := firstTrackerName(t, r)

	// A watcher other than the author, who Redmine adds as watcher anyway.
	watcher := createTestUser(t, r)
	role := strconv.Itoa(firstRoleID(t, r))
	for _, p := range []string{source.Identifier, target.Identifier} {
		r.run(t, "memberships", "create", "--project", p, "--user-id", strconv.Itoa(watcher.ID), "--role-ids", role)
	}

	var original issueState
	r.runJSON(t, &original, "issues", "create", "--project", source.Identifier, "--tracker", tracker,
		"--subject", "E2E copy source", "--watcher", strconv.Itoa(watcher.ID), "--assignee", strconv.Itoa(watcher.ID),
		"--attach", writeAttachFile(t, "copy-"+strconv.FormatInt(time.Now().UnixNano(), 36)+".txt", "data"))
	child := createTestIssueWithSubject(t, r, source.Identifier, "E2E copy child")
	r.run(t, "issues", "update", issueIDArg(child.ID), "--parent", issueIDArg(original.ID))

	t.Run("plain", func(t *testing.T) {
		var copied issueState
		r.runJSON(t, &copied, "issues", "copy", issueIDArg(original.ID))
		got := getIssueState(t, r, copied.ID)
		if got.ID == original.ID || got.Subject != "E2E copy source" || got.Project.ID != source.ID {
			t.Fatalf("copy = %+v", got)
		}
		if len(got.Attachments) != 0 || len(got.Children) != 0 || len(got.Relations) != 0 {
			t.Fatalf("plain copy took attachments/subtasks/relations: %+v", got)
		}
		// Watchers are copied whenever the account may add watchers.
		if !containsWatcherID(got.Watchers, watcher.ID) {
			t.Fatalf("watchers = %+v, want %d", got.Watchers, watcher.ID)
		}
	})

	t.Run("unassigned", func(t *testing.T) {
		var copied issueState
		r.runJSON(t, &copied, "issues", "copy", issueIDArg(original.ID), "--assignee", "none")
		if got := getIssueState(t, r, copied.ID); got.AssignedTo != nil {
			t.Fatalf("assigned_to = %+v, want none", got.AssignedTo)
		}
		// Without --assignee the copy keeps the source's assignee.
		r.runJSON(t, &copied, "issues", "copy", issueIDArg(original.ID))
		if got := getIssueState(t, r, copied.ID); got.AssignedTo == nil || got.AssignedTo.ID != watcher.ID {
			t.Fatalf("assigned_to = %+v, want %d", got.AssignedTo, watcher.ID)
		}
	})

	t.Run("linked", func(t *testing.T) {
		var copied issueState
		r.runJSON(t, &copied, "issues", "copy", issueIDArg(original.ID), "--link")
		got := getIssueState(t, r, copied.ID)
		if len(got.Relations) != 1 || got.Relations[0].RelationType != "copied_to" ||
			got.Relations[0].IssueID != original.ID || got.Relations[0].IssueToID != got.ID {
			t.Fatalf("relations = %+v, want copied_to from #%d", got.Relations, original.ID)
		}
	})

	t.Run("everything", func(t *testing.T) {
		var copied issueState
		r.runJSON(t, &copied, "issues", "copy", issueIDArg(original.ID), "--project", target.Identifier,
			"--subject", "E2E copy target", "--link", "--with-attachments", "--with-subtasks")
		got := getIssueState(t, r, copied.ID)
		if got.Project.ID != target.ID || got.Subject != "E2E copy target" {
			t.Fatalf("copy = %+v", got)
		}
		if len(got.Attachments) != 1 {
			t.Fatalf("attachments = %+v, want 1", got.Attachments)
		}
		if len(got.Children) != 1 || got.Children[0].ID == child.ID {
			t.Fatalf("children = %+v, want one copied subtask", got.Children)
		}
		// Redmine only links copies across projects when cross-project
		// issue relations are allowed (off by default).
		if len(got.Relations) != 0 {
			t.Fatalf("relations = %+v, want none across projects", got.Relations)
		}
		if !containsWatcherID(got.Watchers, watcher.ID) {
			t.Fatalf("watchers = %+v, want %d", got.Watchers, watcher.ID)
		}
	})
}

func TestIssueWatchersAdd_Several(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)
	role := strconv.Itoa(firstRoleID(t, r))
	var users []*userFixture
	for range 2 {
		u := createTestUser(t, r)
		r.run(t, "memberships", "create", "--project", proj.Identifier, "--user-id", strconv.Itoa(u.ID), "--role-ids", role)
		users = append(users, u)
	}

	var env actionEnvelope
	r.runJSON(t, &env, "issues", "watchers", "add", issueIDArg(issue.ID), users[0].Login, strconv.Itoa(users[1].ID))
	if !env.Ok || env.Action != "watched" {
		t.Fatalf("envelope = %+v", env)
	}
	var watchers []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &watchers, "issues", "watchers", "list", issueIDArg(issue.ID))
	for _, u := range users {
		if !containsWatcherID(watchers, u.ID) {
			t.Fatalf("watchers = %+v, missing %d", watchers, u.ID)
		}
	}
}

func TestMCPStdio_IssueWriteSupport(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	source := createTestProject(t, r)
	target := createTestProject(t, r)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	session := r.startMCPStdio(t, ctx, "--enable-writes")

	call := func(name string, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		out, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil {
			t.Fatalf("CallTool %s: %v", name, err)
		}
		return out
	}

	created := call("create_issue", map[string]any{"project_id": source.ID, "subject": "MCP write support", "done_ratio": 30})
	if created.IsError {
		t.Fatalf("create_issue: %s", mcpExtTextContent(t, created))
	}
	id := mcpExtIssueIDFromResult(t, created)
	if got := getIssueState(t, r, id); got.DoneRatio != 30 {
		t.Fatalf("done_ratio = %d, want 30", got.DoneRatio)
	}

	if out := call("update_issue", map[string]any{"id": id, "project_id": target.ID}); out.IsError {
		t.Fatalf("update_issue move: %s", mcpExtTextContent(t, out))
	}
	if got := getIssueState(t, r, id); got.Project.ID != target.ID {
		t.Fatalf("project after move = %+v", got.Project)
	}

	copied := call("copy_issue", map[string]any{"id": id, "subject": "MCP copy", "link_copy": true, "assigned_to_id": 0})
	if copied.IsError {
		t.Fatalf("copy_issue: %s", mcpExtTextContent(t, copied))
	}
	copyID := mcpExtIssueIDFromResult(t, copied)
	got := getIssueState(t, r, copyID)
	if got.Subject != "MCP copy" || got.Project.ID != target.ID || len(got.Relations) != 1 || got.AssignedTo != nil {
		t.Fatalf("copy = %+v", got)
	}

	issueID, filename, _ := createIssueWithAttachment(t, r, source.Identifier)
	att := findIssueAttachmentID(t, r, issueID, filename)
	if out := call("update_issue", map[string]any{"id": issueID, "deleted_attachment_ids": []int{att.ID}}); out.IsError {
		t.Fatalf("update_issue deleted_attachment_ids: %s", mcpExtTextContent(t, out))
	}
	if got := getIssueState(t, r, issueID); len(got.Attachments) != 0 {
		raw, _ := json.Marshal(got.Attachments)
		t.Fatalf("attachments after deleted_attachment_ids = %s", raw)
	}
}
