//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// issueAssignee is the assigned_to shape returned by `issues get`.
type issueAssignee struct {
	AssignedTo *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"assigned_to"`
}

func getIssueAssignee(t *testing.T, r *cliRunner, issueID int) issueAssignee {
	t.Helper()
	var got issueAssignee
	r.runJSON(t, &got, "issues", "get", issueIDArg(issueID))
	return got
}

func requireAssignedTo(t *testing.T, r *cliRunner, issueID, wantID int) {
	t.Helper()
	got := getIssueAssignee(t, r, issueID)
	if got.AssignedTo == nil || got.AssignedTo.ID != wantID {
		t.Fatalf("issue %d assigned_to = %+v, want ID %d", issueID, got.AssignedTo, wantID)
	}
}

func requireUnassigned(t *testing.T, r *cliRunner, issueID int) {
	t.Helper()
	got := getIssueAssignee(t, r, issueID)
	if got.AssignedTo != nil {
		t.Fatalf("issue %d assigned_to = %+v, want unassigned", issueID, got.AssignedTo)
	}
}

func addProjectMember(t *testing.T, r *cliRunner, project, flag string, principalID, roleID int) {
	t.Helper()
	var created struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &created, "memberships", "create",
		"--project", project,
		flag, strconv.Itoa(principalID),
		"--role-ids", strconv.Itoa(roleID))
	if created.ID == 0 {
		t.Fatalf("membership for %s %d not created", flag, principalID)
	}
}

// TestAssignee_GroupByName verifies that a group member of the project can
// be assigned by name through `issues update`, `issues assign` and used as
// an `issues list` filter. Requires issue_group_assignment, which the
// bootstrap enables.
func TestAssignee_GroupByName(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	grp := createTestGroup(t, r)
	addProjectMember(t, r, proj.Identifier, "--group-id", grp.ID, firstRoleID(t, r))
	issue := createTestIssue(t, r, proj.Identifier)

	var env actionEnvelope
	r.runJSON(t, &env, "issues", "update", issueIDArg(issue.ID), "--assignee", grp.Name)
	if !env.Ok {
		t.Fatalf("unexpected update envelope: %+v", env)
	}
	requireAssignedTo(t, r, issue.ID, grp.ID)

	ids := listIssueIDs(t, r, "--project", proj.Identifier, "--assignee", grp.Name, "--limit", "100")
	if !containsInt(ids, issue.ID) {
		t.Fatalf("--assignee %q should include issue %d; got %v", grp.Name, issue.ID, ids)
	}

	r.runJSON(t, &env, "issues", "assign", issueIDArg(issue.ID), "none")
	if !env.Ok || env.Action != "unassigned" || env.Resource != "issue" {
		t.Fatalf("unexpected unassign envelope: %+v", env)
	}
	requireUnassigned(t, r, issue.ID)

	// Case-insensitive name match through `issues assign`.
	r.runJSON(t, &env, "issues", "assign", issueIDArg(issue.ID), strings.ToUpper(grp.Name))
	if !env.Ok || env.Action != "assigned" {
		t.Fatalf("unexpected assign envelope: %+v", env)
	}
	requireAssignedTo(t, r, issue.ID, grp.ID)
}

// TestAssignee_NoneUnassignsAndFilters covers `--assignee none` on update,
// create and list.
func TestAssignee_NoneUnassignsAndFilters(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	assigned := createTestIssueWithSubject(t, r, proj.Identifier, "Assigned issue for none filter")
	unassigned := createTestIssueWithSubject(t, r, proj.Identifier, "Unassigned issue for none filter")

	var env actionEnvelope
	r.runJSON(t, &env, "issues", "assign", issueIDArg(assigned.ID), "me")
	if !env.Ok {
		t.Fatalf("assign me failed: %+v", env)
	}

	ids := listIssueIDs(t, r, "--project", proj.Identifier, "--assignee", "none", "--limit", "100")
	if !containsInt(ids, unassigned.ID) || containsInt(ids, assigned.ID) {
		t.Fatalf("--assignee none = %v, want %d and not %d", ids, unassigned.ID, assigned.ID)
	}

	r.runJSON(t, &env, "issues", "update", issueIDArg(assigned.ID), "--assignee", "none")
	if !env.Ok {
		t.Fatalf("update --assignee none failed: %+v", env)
	}
	requireUnassigned(t, r, assigned.ID)

	ids = listIssueIDs(t, r, "--project", proj.Identifier, "--assignee", "none", "--limit", "100")
	if !containsInt(ids, unassigned.ID) || !containsInt(ids, assigned.ID) {
		t.Fatalf("--assignee none after unassign = %v, want both %d and %d", ids, unassigned.ID, assigned.ID)
	}

	var created struct {
		ID         int `json:"id"`
		AssignedTo *struct {
			ID int `json:"id"`
		} `json:"assigned_to"`
	}
	r.runJSON(t, &created, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", firstTrackerName(t, r),
		"--subject", "Created with --assignee none",
		"--assignee", "none")
	if created.ID == 0 || created.AssignedTo != nil {
		t.Fatalf("create --assignee none = %+v, want unassigned issue", created)
	}
}

// TestAssignee_MCPUnassignAndNoneFilter covers MCP parity: update_issue with
// assigned_to_id 0 clears the assignee and list_issues accepts "none".
func TestAssignee_MCPUnassignAndNoneFilter(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)

	var env actionEnvelope
	r.runJSON(t, &env, "issues", "assign", issueIDArg(issue.ID), "me")
	if !env.Ok {
		t.Fatalf("assign me failed: %+v", env)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	session := r.startMCPStdio(t, ctx, "--enable-writes")

	listUnassigned := func() []int {
		t.Helper()
		out, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "list_issues",
			Arguments: map[string]any{"project_id": proj.Identifier, "assigned_to_id": "none"},
		})
		if err != nil || out.IsError {
			t.Fatalf("list_issues: err=%v result=%s", err, structuredJSON(out))
		}
		v, _ := mcpExtFieldFromResult(t, out, "issues")
		raw, _ := json.Marshal(v)
		var issues []struct {
			ID int `json:"id"`
		}
		if err := json.Unmarshal(raw, &issues); err != nil {
			t.Fatalf("decode issues: %v (%s)", err, raw)
		}
		ids := make([]int, len(issues))
		for i, it := range issues {
			ids[i] = it.ID
		}
		return ids
	}

	if ids := listUnassigned(); containsInt(ids, issue.ID) {
		t.Fatalf("list_issues assigned_to_id=none returned assigned issue %d: %v", issue.ID, ids)
	}

	out, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "update_issue",
		Arguments: map[string]any{"id": issue.ID, "assigned_to_id": 0},
	})
	if err != nil || out.IsError {
		t.Fatalf("update_issue assigned_to_id=0: err=%v result=%s", err, structuredJSON(out))
	}
	requireUnassigned(t, r, issue.ID)

	if ids := listUnassigned(); !containsInt(ids, issue.ID) {
		t.Fatalf("list_issues assigned_to_id=none should include %d after unassign: %v", issue.ID, ids)
	}
}

// TestAssignee_NonAdminResolvesProjectMembers drives the CLI as a regular
// project member. /users.json is admin-only, so before membership-based
// resolution none of these name lookups worked for non-admins.
func TestAssignee_NonAdminResolvesProjectMembers(t *testing.T) {
	requireE2E(t)
	admin := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, admin)
	member := createTestUser(t, admin)
	colleague := createTestUser(t, admin)
	grp := createTestGroup(t, admin)
	roleID := firstRoleID(t, admin)
	addProjectMember(t, admin, proj.Identifier, "--user-id", member.ID, roleID)
	addProjectMember(t, admin, proj.Identifier, "--user-id", colleague.ID, roleID)
	addProjectMember(t, admin, proj.Identifier, "--group-id", grp.ID, roleID)
	issue := createTestIssue(t, admin, proj.Identifier)

	r := newCLIRunnerBasicAuth(t, e2eBaseURL(), member.Login, member.Password)

	// Sanity check: this account really cannot list users.
	if _, _, err := r.runRaw("users", "list"); err == nil {
		t.Fatal("expected `users list` to fail for a non-admin account")
	}

	var env actionEnvelope
	r.runJSON(t, &env, "issues", "update", issueIDArg(issue.ID), "--assignee", colleague.Name)
	if !env.Ok {
		t.Fatalf("non-admin update --assignee by name failed: %+v", env)
	}
	requireAssignedTo(t, admin, issue.ID, colleague.ID)

	ids := listIssueIDs(t, r, "--project", proj.Identifier, "--assignee", colleague.Name, "--limit", "100")
	if !containsInt(ids, issue.ID) {
		t.Fatalf("non-admin --assignee %q should include issue %d; got %v", colleague.Name, issue.ID, ids)
	}

	r.runJSON(t, &env, "issues", "assign", issueIDArg(issue.ID), grp.Name)
	if !env.Ok || env.Action != "assigned" {
		t.Fatalf("non-admin assign to group failed: %+v", env)
	}
	requireAssignedTo(t, admin, issue.ID, grp.ID)

	var created struct {
		ID         int `json:"id"`
		AssignedTo *struct {
			ID int `json:"id"`
		} `json:"assigned_to"`
	}
	r.runJSON(t, &created, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", firstTrackerName(t, admin),
		"--subject", "Created by non-admin with assignee name",
		"--assignee", colleague.Name)
	if created.AssignedTo == nil || created.AssignedTo.ID != colleague.ID {
		t.Fatalf("non-admin create --assignee = %+v, want user %d", created.AssignedTo, colleague.ID)
	}

	// Logins need the admin-only users endpoint; the error must point at the
	// project members instead of a bare permission failure.
	stdout, stderr := r.runExpectError(t, "issues", "update", issueIDArg(issue.ID), "--assignee", colleague.Login)
	combined := string(stdout) + string(stderr)
	for _, want := range []string{"Available project members", colleague.Name, grp.Name + " [group]", "admin privileges"} {
		if !strings.Contains(combined, want) {
			t.Errorf("login resolution error missing %q\nstdout:\n%s\nstderr:\n%s", want, stdout, stderr)
		}
	}
}
