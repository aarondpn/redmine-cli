//go:build e2e

package e2e

import (
	"context"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type categoryJSON struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Project struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"project"`
	AssignedTo *struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"assigned_to"`
}

func createCategoryCLI(t *testing.T, r *cliRunner, project, name string, extra ...string) categoryJSON {
	t.Helper()
	var cat categoryJSON
	args := append([]string{"categories", "create", "--project", project, "--name", name}, extra...)
	r.runJSON(t, &cat, args...)
	if cat.ID == 0 || cat.Name != name {
		t.Fatalf("categories create = %+v, want name %q", cat, name)
	}
	return cat
}

func listCategoryIDs(t *testing.T, r *cliRunner, project string) []int {
	t.Helper()
	var listed []categoryJSON
	r.runJSON(t, &listed, "categories", "list", "--project", project)
	ids := make([]int, len(listed))
	for i, c := range listed {
		ids[i] = c.ID
	}
	return ids
}

func issueCategoryID(t *testing.T, r *cliRunner, issueID int) int {
	t.Helper()
	var issue struct {
		Category *struct {
			ID int `json:"id"`
		} `json:"category"`
	}
	r.runJSON(t, &issue, "issues", "get", issueIDArg(issueID))
	if issue.Category == nil {
		return 0
	}
	return issue.Category.ID
}

// TestCategories_Lifecycle covers create (with a default assignee), get by ID
// and by name, update (rename, clear the assignee) and delete.
func TestCategories_Lifecycle(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	name := "E2E Lifecycle " + strconv.FormatInt(time.Now().UnixNano(), 36)
	cat := createCategoryCLI(t, r, proj.Identifier, name, "--assignee", "me")
	if cat.AssignedTo == nil {
		t.Fatalf("create --assignee me returned no assignee: %+v", cat)
	}

	var byID categoryJSON
	r.runJSON(t, &byID, "categories", "get", strconv.Itoa(cat.ID))
	if byID.ID != cat.ID || byID.Name != name || byID.AssignedTo == nil || byID.AssignedTo.ID != cat.AssignedTo.ID {
		t.Fatalf("get by id = %+v, want %+v", byID, cat)
	}
	var byName categoryJSON
	r.runJSON(t, &byName, "categories", "get", name, "--project", proj.Identifier)
	if byName.ID != cat.ID {
		t.Fatalf("get by name = %+v, want id %d", byName, cat.ID)
	}

	renamed := name + " renamed"
	var env actionEnvelope
	r.runJSON(t, &env, "categories", "update", strconv.Itoa(cat.ID), "--name", renamed, "--assignee", "none")
	if !env.Ok || env.Action != "updated" || env.Resource != "category" || envelopeIntID(env.ID) != cat.ID {
		t.Fatalf("update envelope = %+v", env)
	}
	var after categoryJSON
	r.runJSON(t, &after, "categories", "get", strconv.Itoa(cat.ID))
	if after.Name != renamed || after.AssignedTo != nil {
		t.Fatalf("after update = %+v, want name %q and no assignee", after, renamed)
	}

	// Changing only the name keeps the (cleared) assignee untouched.
	r.runJSON(t, &env, "categories", "update", renamed, "--project", proj.Identifier, "--name", name)
	r.runJSON(t, &after, "categories", "get", strconv.Itoa(cat.ID))
	if after.Name != name {
		t.Fatalf("rename by name failed: %+v", after)
	}

	r.runJSON(t, &env, "categories", "delete", strconv.Itoa(cat.ID), "--force")
	if !env.Ok || env.Action != "deleted" || env.Resource != "category" || envelopeIntID(env.ID) != cat.ID {
		t.Fatalf("delete envelope = %+v", env)
	}
	if containsInt(listCategoryIDs(t, r, proj.Identifier), cat.ID) {
		t.Fatalf("category %d still listed after delete", cat.ID)
	}
	assertErrorCode(t, r.runExpectJSONError(t, "categories", "get", strconv.Itoa(cat.ID)), "not_found")
}

// TestCategories_DeleteCancelled pins that declining the confirmation prompt
// (empty stdin) leaves the category in place.
func TestCategories_DeleteCancelled(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	cat := createCategoryCLI(t, r, proj.Identifier, "E2E Keep")

	var env actionEnvelope
	r.runJSON(t, &env, "categories", "delete", strconv.Itoa(cat.ID))
	if env.Ok || env.Action != "deleted" {
		t.Fatalf("cancelled delete envelope = %+v", env)
	}
	if !containsInt(listCategoryIDs(t, r, proj.Identifier), cat.ID) {
		t.Fatalf("category %d vanished despite cancelled delete", cat.ID)
	}
}

// TestCategories_DeleteReassign verifies --reassign-to moves the issues, and
// that a plain delete leaves them without a category.
func TestCategories_DeleteReassign(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	a := createCategoryCLI(t, r, proj.Identifier, "E2E Cat A")
	b := createCategoryCLI(t, r, proj.Identifier, "E2E Cat B")

	var created struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &created, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", firstTrackerName(t, r),
		"--subject", "E2E categorized issue",
		"--category", a.Name)
	if got := issueCategoryID(t, r, created.ID); got != a.ID {
		t.Fatalf("issue category = %d, want %d", got, a.ID)
	}

	// Name-based --reassign-to resolves in the deleted category's project.
	var env actionEnvelope
	r.runJSON(t, &env, "categories", "delete", strconv.Itoa(a.ID), "--reassign-to", b.Name, "--force")
	if !env.Ok || !strings.Contains(env.Message, strconv.Itoa(b.ID)) {
		t.Fatalf("delete envelope = %+v, want message naming category %d", env, b.ID)
	}
	if got := issueCategoryID(t, r, created.ID); got != b.ID {
		t.Fatalf("issue category after reassign = %d, want %d", got, b.ID)
	}

	r.runJSON(t, &env, "categories", "delete", strconv.Itoa(b.ID), "--force")
	if !env.Ok {
		t.Fatalf("delete B envelope = %+v", env)
	}
	if got := issueCategoryID(t, r, created.ID); got != 0 {
		t.Fatalf("issue category after plain delete = %d, want none", got)
	}
}

// TestCategories_DeleteReassignOtherProject verifies the CLI refuses a
// reassign target from another project (Redmine would silently ignore it).
func TestCategories_DeleteReassignOtherProject(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	other := createTestProject(t, r)

	mine := createCategoryCLI(t, r, proj.Identifier, "E2E Mine")
	foreign := createCategoryCLI(t, r, other.Identifier, "E2E Foreign")

	stderr := r.runExpectJSONError(t, "categories", "delete", strconv.Itoa(mine.ID),
		"--reassign-to", strconv.Itoa(foreign.ID), "--force")
	if !strings.Contains(string(stderr), "belongs to project") {
		t.Fatalf("stderr does not explain the rejection:\n%s", stderr)
	}
	if !containsInt(listCategoryIDs(t, r, proj.Identifier), mine.ID) {
		t.Fatalf("category %d was deleted despite the rejected reassign", mine.ID)
	}
	if !containsInt(listCategoryIDs(t, r, other.Identifier), foreign.ID) {
		t.Fatalf("foreign category %d disappeared", foreign.ID)
	}

	// Reassigning to the category itself is rejected before any request.
	r.runExpectJSONError(t, "categories", "delete", strconv.Itoa(mine.ID),
		"--reassign-to", strconv.Itoa(mine.ID), "--force")
}

func TestCategories_Validation(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	createCategoryCLI(t, r, proj.Identifier, "E2E Dup")
	assertErrorCode(t, r.runExpectJSONError(t, "categories", "create",
		"--project", proj.Identifier, "--name", "E2E Dup"), "validation_failed")

	// Nothing to update is rejected client-side.
	r.runExpectJSONError(t, "categories", "update", "1")
}

// TestMCPStdio_CategoryTools drives get/create/update/delete_issue_category
// over MCP, including reassignment and the cross-project guard.
func TestMCPStdio_CategoryTools(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	other := createTestProject(t, r)
	foreign := createCategoryCLI(t, r, other.Identifier, "E2E MCP Foreign")

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
	mustOK := func(name string, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		out := call(name, args)
		if out.IsError {
			t.Fatalf("%s returned IsError: %s", name, structuredJSON(out))
		}
		return out
	}
	idOf := func(out *mcp.CallToolResult) int {
		t.Helper()
		v, ok := mcpExtFieldFromResult(t, out, "id")
		f, isNum := v.(float64)
		if !ok || !isNum || f == 0 {
			t.Fatalf("no id in result: %s", structuredJSON(out))
		}
		return int(f)
	}

	a := idOf(mustOK("create_issue_category", map[string]any{"project_id": proj.Identifier, "name": "MCP A"}))
	b := idOf(mustOK("create_issue_category", map[string]any{"project_id": proj.Identifier, "name": "MCP B", "assigned_to_id": envelopeAdminID(t, r)}))

	got := mustOK("get_issue_category", map[string]any{"id": b})
	if name, _ := mcpExtFieldFromResult(t, got, "name"); name != "MCP B" {
		t.Fatalf("get_issue_category = %s", structuredJSON(got))
	}
	if _, ok := mcpExtFieldFromResult(t, got, "assigned_to"); !ok {
		t.Fatalf("expected assigned_to in %s", structuredJSON(got))
	}

	mustOK("update_issue_category", map[string]any{"id": b, "name": "MCP B2", "assigned_to_id": 0})
	var after categoryJSON
	r.runJSON(t, &after, "categories", "get", strconv.Itoa(b))
	if after.Name != "MCP B2" || after.AssignedTo != nil {
		t.Fatalf("after update_issue_category = %+v", after)
	}

	if out := call("update_issue_category", map[string]any{"id": b}); !out.IsError {
		t.Fatalf("update with no fields should be an error: %s", structuredJSON(out))
	}

	var created struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &created, "issues", "create", "--project", proj.Identifier,
		"--tracker", firstTrackerName(t, r), "--subject", "E2E MCP categorized", "--category", strconv.Itoa(a))

	if out := call("delete_issue_category", map[string]any{"id": a, "reassign_to_id": foreign.ID}); !out.IsError {
		t.Fatalf("cross-project reassign should be an error: %s", structuredJSON(out))
	}
	if !containsInt(listCategoryIDs(t, r, proj.Identifier), a) {
		t.Fatalf("category %d deleted despite rejected reassign", a)
	}

	mustOK("delete_issue_category", map[string]any{"id": a, "reassign_to_id": b})
	if got := issueCategoryID(t, r, created.ID); got != b {
		t.Fatalf("issue category after MCP reassign = %d, want %d", got, b)
	}
	mustOK("delete_issue_category", map[string]any{"id": b})
	if got := issueCategoryID(t, r, created.ID); got != 0 {
		t.Fatalf("issue category after MCP delete = %d, want none", got)
	}
}

// envelopeAdminID returns the numeric ID of the user the e2e runner acts as.
func envelopeAdminID(t *testing.T, r *cliRunner) int {
	t.Helper()
	var me struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &me, "users", "me")
	if me.ID == 0 {
		t.Fatal("users me returned no id")
	}
	return me.ID
}
