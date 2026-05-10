//go:build e2e

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestMCPStdio_WriteToolsExposedAndCallable verifies that --enable-writes
// registers the destructive tools and that create_project / delete_project
// actually round-trip against the live Redmine instance.
func TestMCPStdio_WriteToolsExposedAndCallable(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := r.startMCPStdio(t, ctx, "--enable-writes")

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := toolNames(tools.Tools)
	mustContain(t, names,
		"create_issue", "update_issue", "delete_issue",
		"create_project", "delete_project",
	)

	identifier := uniqueIdentifier(t)

	createOut, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "create_project",
		Arguments: map[string]any{
			"name":        "MCP Ext " + identifier,
			"identifier":  identifier,
			"description": "Created by mcp_extended_test.go",
		},
	})
	if err != nil {
		t.Fatalf("CallTool create_project: %v", err)
	}
	if createOut.IsError {
		t.Fatalf("create_project returned IsError: %s", structuredJSON(createOut))
	}

	// Register a fallback cleanup. The explicit delete below is the one
	// under test; this cleanup only fires if that call (or anything between)
	// fails, so the project is never left behind.
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cleanupCancel()
		_, _ = session.CallTool(cleanupCtx, &mcp.CallToolParams{
			Name:      "delete_project",
			Arguments: map[string]any{"identifier": identifier},
		})
	})

	delOut, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "delete_project",
		Arguments: map[string]any{"identifier": identifier},
	})
	if err != nil {
		t.Fatalf("CallTool delete_project: %v", err)
	}
	if delOut.IsError {
		t.Fatalf("delete_project returned IsError: %s", structuredJSON(delOut))
	}
}

// TestMCPStdio_CreateAndUpdateIssueViaTool exercises the full create_issue /
// update_issue path via the MCP tool surface, then verifies the change with
// get_issue. The parent project is created via the CLI fixture so its
// teardown also deletes the issue.
func TestMCPStdio_CreateAndUpdateIssueViaTool(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := r.startMCPStdio(t, ctx, "--enable-writes")

	originalSubject := fmt.Sprintf("MCP create %d", time.Now().UnixNano())
	createOut, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "create_issue",
		Arguments: map[string]any{
			"project_id":  proj.ID,
			"subject":     originalSubject,
			"description": "Created via MCP create_issue tool",
		},
	})
	if err != nil {
		t.Fatalf("CallTool create_issue: %v", err)
	}
	if createOut.IsError {
		t.Fatalf("create_issue returned IsError: %s", structuredJSON(createOut))
	}

	issueID := mcpExtIssueIDFromResult(t, createOut)
	if issueID == 0 {
		t.Fatalf("create_issue returned no usable id; payload: %s", structuredJSON(createOut))
	}

	updatedSubject := originalSubject + " (updated)"
	updateOut, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "update_issue",
		Arguments: map[string]any{
			"id":      issueID,
			"subject": updatedSubject,
		},
	})
	if err != nil {
		t.Fatalf("CallTool update_issue: %v", err)
	}
	if updateOut.IsError {
		t.Fatalf("update_issue returned IsError: %s", structuredJSON(updateOut))
	}

	getOut, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_issue",
		Arguments: map[string]any{"id": issueID},
	})
	if err != nil {
		t.Fatalf("CallTool get_issue: %v", err)
	}
	if getOut.IsError {
		t.Fatalf("get_issue returned IsError: %s", structuredJSON(getOut))
	}
	if got := mcpExtIssueSubjectFromResult(t, getOut); got != updatedSubject {
		t.Fatalf("issue subject after update = %q, want %q (payload: %s)",
			got, updatedSubject, structuredJSON(getOut))
	}
}

// TestMCPStdio_MultipleEnableGroups verifies that multiple groups passed via
// --enable-groups are all exposed and that an unrelated group is hidden.
func TestMCPStdio_MultipleEnableGroups(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := r.startMCPStdio(t, ctx, "--enable-groups", "issues,time")

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := toolNames(tools.Tools)
	mustContain(t, names, "list_issues", "get_issue", "list_time_entries", "summary_time_entries")

	for _, name := range names {
		switch name {
		case "list_wiki_pages", "get_wiki_page", "create_wiki_page",
			"list_projects", "get_project", "list_users":
			t.Errorf("--enable-groups issues,time leaked %q from another group", name)
		}
	}
}

func TestMCPStdio_GetTrackerTool(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := r.startMCPStdio(t, ctx)

	listOut, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_trackers"})
	if err != nil {
		t.Fatalf("CallTool list_trackers: %v", err)
	}
	if listOut.IsError {
		t.Fatalf("list_trackers returned IsError: %s", structuredJSON(listOut))
	}

	var listed struct {
		Trackers []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"trackers"`
	}
	if err := json.Unmarshal([]byte(mcpExtTextContent(t, listOut)), &listed); err != nil {
		t.Fatalf("decode list_trackers: %v\npayload:\n%s", err, structuredJSON(listOut))
	}
	if len(listed.Trackers) == 0 {
		t.Fatal("list_trackers returned no trackers")
	}

	getOut, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_tracker",
		Arguments: map[string]any{"id": listed.Trackers[0].ID},
	})
	if err != nil {
		t.Fatalf("CallTool get_tracker: %v", err)
	}
	if getOut.IsError {
		t.Fatalf("get_tracker returned IsError: %s", structuredJSON(getOut))
	}

	var got struct {
		ID            int    `json:"id"`
		Name          string `json:"name"`
		DefaultStatus struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"default_status"`
	}
	if err := json.Unmarshal([]byte(mcpExtTextContent(t, getOut)), &got); err != nil {
		t.Fatalf("decode get_tracker: %v\npayload:\n%s", err, structuredJSON(getOut))
	}
	if got.ID != listed.Trackers[0].ID || got.Name != listed.Trackers[0].Name {
		t.Fatalf("get_tracker returned %+v, want id=%d name=%q", got, listed.Trackers[0].ID, listed.Trackers[0].Name)
	}
	if got.DefaultStatus.ID == 0 || got.DefaultStatus.Name == "" {
		t.Fatalf("get_tracker missing default status: %+v", got.DefaultStatus)
	}
}

// TestMCPStdio_DisableGroupOverridesEnable pins the precedence rule from
// internal/cmd/mcp/serve.go: --disable-groups wins over --enable-groups when
// both name the same group.
func TestMCPStdio_DisableGroupOverridesEnable(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := r.startMCPStdio(t, ctx, "--enable-groups", "issues", "--disable-groups", "issues")

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	for _, name := range toolNames(tools.Tools) {
		switch name {
		case "list_issues", "get_issue", "create_issue", "update_issue",
			"delete_issue", "close_issue", "reopen_issue", "add_issue_comment":
			t.Errorf("issues tool %q exposed despite --disable-groups issues", name)
		}
	}
}

// TestMCPStdio_InvalidGroupName ensures unknown group names cause the server
// to fail fast: the subprocess exits before completing the MCP handshake, so
// the client's Connect call returns an error.
func TestMCPStdio_InvalidGroupName(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := mcpExtTryConnectStdio(ctx, r, "--enable-groups", "this-group-does-not-exist"); err == nil {
		t.Fatal("MCP server accepted connection with unknown group; expected failure")
	}
}

// TestMCPStdio_EnableToolsAllowList verifies that --enable-tools restricts the
// catalog to exactly the named tools.
func TestMCPStdio_EnableToolsAllowList(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := r.startMCPStdio(t, ctx, "--enable-tools", "list_projects,get_project")

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := toolNames(tools.Tools)
	want := map[string]bool{"list_projects": false, "get_project": false}
	for _, name := range names {
		if _, ok := want[name]; ok {
			want[name] = true
			continue
		}
		t.Errorf("--enable-tools allow-list leaked unrelated tool %q", name)
	}
	for name, seen := range want {
		if !seen {
			t.Errorf("--enable-tools allow-list missing %q (got %v)", name, names)
		}
	}
}

// TestMCPStdio_DisableToolsDenyList verifies that --disable-tools removes the
// named tool while leaving sibling read tools registered.
func TestMCPStdio_DisableToolsDenyList(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := r.startMCPStdio(t, ctx, "--disable-tools", "list_projects")

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}
	names := toolNames(tools.Tools)
	for _, name := range names {
		if name == "list_projects" {
			t.Errorf("list_projects exposed despite --disable-tools list_projects")
		}
	}
	mustContain(t, names, "get_project", "list_issues", "get_issue")
}

// TestMCPStdio_CallToolAgainstNonexistent verifies that a call against a
// clearly-bogus issue ID surfaces a structured 404 / not-found error rather
// than a transport-level failure.
func TestMCPStdio_CallToolAgainstNonexistent(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := r.startMCPStdio(t, ctx)

	out, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "get_issue",
		Arguments: map[string]any{"id": 2147483600},
	})
	if err != nil {
		t.Fatalf("CallTool get_issue (bogus id): %v", err)
	}
	if !out.IsError {
		t.Fatalf("expected IsError=true for bogus issue id; payload: %s", structuredJSON(out))
	}
	body := strings.ToLower(structuredJSON(out))
	if !strings.Contains(body, "404") && !strings.Contains(body, "not found") && !strings.Contains(body, "notfound") {
		t.Errorf("structured content does not mention 404/not-found; got %s", structuredJSON(out))
	}
}

// TestMCPStdio_CallToolUnknownName verifies that calling a tool the server
// never registered surfaces as an error to the client.
func TestMCPStdio_CallToolUnknownName(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := r.startMCPStdio(t, ctx)

	out, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name: "nonexistent_tool",
	})
	if err != nil {
		// Transport-level error is acceptable; the SDK reports unknown
		// tools as JSON-RPC errors. Sanity-check the message.
		if !strings.Contains(strings.ToLower(err.Error()), "unknown tool") &&
			!strings.Contains(strings.ToLower(err.Error()), "not found") &&
			!strings.Contains(strings.ToLower(err.Error()), "nonexistent_tool") {
			t.Errorf("unknown-tool error message lacks diagnostic detail: %v", err)
		}
		return
	}
	if !out.IsError {
		t.Fatalf("expected IsError=true (or call error) for unknown tool; payload: %s", structuredJSON(out))
	}
	body := strings.ToLower(structuredJSON(out))
	if !strings.Contains(body, "unknown") && !strings.Contains(body, "not found") &&
		!strings.Contains(body, "nonexistent_tool") {
		t.Errorf("structured error lacks diagnostic detail: %s", structuredJSON(out))
	}
}

// TestMCPStdio_ToolCatalogStable snapshots the read-only tool catalog so
// accidental tool removal / rename surfaces in review. The golden file lives
// at e2e/testdata/mcp_tools_readonly.golden.json and is refreshed via
// UPDATE_GOLDENS=1.
func TestMCPStdio_ToolCatalogStable(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	session := r.startMCPStdio(t, ctx)

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	type catalogEntry struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	catalog := make([]catalogEntry, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		catalog = append(catalog, catalogEntry{Name: tool.Name, Description: tool.Description})
	}
	sort.Slice(catalog, func(i, j int) bool { return catalog[i].Name < catalog[j].Name })

	assertGoldenJSON(t, "testdata/mcp_tools_readonly.golden.json", catalog)
}

// mcpExtTryConnectStdio mirrors startMCPStdio but returns an error instead of
// failing the test. Used to assert the server refuses to start under invalid
// flag combinations.
func mcpExtTryConnectStdio(ctx context.Context, r *cliRunner, extraArgs ...string) error {
	cmd := exec.Command(builtCLIPath, r.mcpServerArgs(extraArgs...)...)
	cmd.Dir = r.repoRoot
	cmd.Env = append(os.Environ(), "REDMINE_NO_UPDATE_CHECK=1")

	transport := &mcp.CommandTransport{Command: cmd}
	client := mcp.NewClient(&mcp.Implementation{Name: "redmine-cli-e2e", Version: "v0"}, nil)

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	session, err := client.Connect(connectCtx, transport, nil)
	if err != nil {
		return err
	}
	_ = session.Close()
	return nil
}

// mcpExtIssueIDFromResult pulls the new issue ID out of a CallTool result.
// The structured content for create_issue is a *models.Issue; the JSON shape
// has "id" at the top level.
func mcpExtIssueIDFromResult(t *testing.T, out *mcp.CallToolResult) int {
	t.Helper()
	v, ok := mcpExtFieldFromResult(t, out, "id")
	if !ok {
		return 0
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	default:
		return 0
	}
}

// mcpExtIssueSubjectFromResult pulls the subject out of a get_issue result.
func mcpExtIssueSubjectFromResult(t *testing.T, out *mcp.CallToolResult) string {
	t.Helper()
	v, ok := mcpExtFieldFromResult(t, out, "subject")
	if !ok {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func mcpExtTextContent(t *testing.T, out *mcp.CallToolResult) string {
	t.Helper()
	if len(out.Content) == 0 {
		t.Fatal("call tool result contained no content blocks")
	}
	tc, ok := out.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("expected TextContent, got %T", out.Content[0])
	}
	return tc.Text
}

// mcpExtFieldFromResult returns the named top-level field from a CallToolResult.
// Prefers StructuredContent; falls back to parsing textual content blocks as
// JSON. The boolean reports whether the field was found at all.
func mcpExtFieldFromResult(t *testing.T, out *mcp.CallToolResult, field string) (any, bool) {
	t.Helper()

	if out.StructuredContent != nil {
		raw, err := json.Marshal(out.StructuredContent)
		if err != nil {
			t.Fatalf("marshal structured content: %v", err)
		}
		var obj map[string]any
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatalf("decode structured content: %v\npayload: %s", err, raw)
		}
		if v, ok := obj[field]; ok {
			return v, true
		}
	}

	for _, block := range out.Content {
		tc, ok := block.(*mcp.TextContent)
		if !ok {
			continue
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(tc.Text), &obj); err != nil {
			continue
		}
		if v, ok := obj[field]; ok {
			return v, true
		}
	}
	return nil, false
}
