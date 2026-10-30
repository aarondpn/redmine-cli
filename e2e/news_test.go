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

type newsJSON struct {
	ID      int `json:"id"`
	Project struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"project"`
	Author struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"author"`
	Title       string `json:"title"`
	Summary     string `json:"summary"`
	Description string `json:"description"`
	CreatedOn   string `json:"created_on"`
	Attachments []struct {
		ID       int    `json:"id"`
		Filename string `json:"filename"`
	} `json:"attachments"`
	Comments []struct {
		ID      int    `json:"id"`
		Content string `json:"content"`
	} `json:"comments"`
}

// createNewsCLI creates a news item and returns the ID the read-back found.
func createNewsCLI(t *testing.T, r *cliRunner, project, title string, extra ...string) int {
	t.Helper()
	var env actionEnvelope
	args := append([]string{"news", "create", "--project", project, "--title", title}, extra...)
	r.runJSON(t, &env, args...)
	id := envelopeIntID(env.ID)
	if !env.Ok || env.Action != "created" || env.Resource != "news" || id == 0 {
		t.Fatalf("news create envelope = %+v, want created news with an id", env)
	}
	return id
}

func listNewsIDs(t *testing.T, r *cliRunner, args ...string) []int {
	t.Helper()
	var items []newsJSON
	r.runJSON(t, &items, append([]string{"news", "list"}, args...)...)
	ids := make([]int, len(items))
	for i, n := range items {
		ids[i] = n.ID
	}
	return ids
}

func TestNews_Lifecycle(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	title := "E2E news " + strconv.FormatInt(time.Now().UnixNano(), 36)
	id := createNewsCLI(t, r, proj.Identifier, title, "--summary", "e2e summary", "--description", "e2e description")

	if !containsInt(listNewsIDs(t, r, "--project", proj.Identifier), id) {
		t.Fatalf("project news list does not contain %d", id)
	}
	if !containsInt(listNewsIDs(t, r, "--limit", "100"), id) {
		t.Fatalf("global news list does not contain %d", id)
	}

	var got newsJSON
	r.runJSON(t, &got, "news", "get", strconv.Itoa(id))
	if got.ID != id || got.Title != title || got.Summary != "e2e summary" || got.Description != "e2e description" ||
		got.Project.ID != proj.ID || got.Author.ID == 0 || got.CreatedOn == "" {
		t.Fatalf("news get = %+v", got)
	}

	// Update only the title: summary and description stay.
	var upd actionEnvelope
	r.runJSON(t, &upd, "news", "update", strconv.Itoa(id), "--title", title+" v2")
	if !upd.Ok || upd.Action != "updated" || upd.Resource != "news" || envelopeIntID(upd.ID) != id {
		t.Fatalf("news update envelope = %+v", upd)
	}
	r.runJSON(t, &got, "news", "get", strconv.Itoa(id))
	if got.Title != title+" v2" || got.Summary != "e2e summary" || got.Description != "e2e description" {
		t.Fatalf("after title update = %+v", got)
	}

	// An empty --summary clears it.
	r.runJSON(t, &upd, "news", "update", strconv.Itoa(id), "--summary", "", "--description", "changed")
	got = newsJSON{}
	r.runJSON(t, &got, "news", "get", strconv.Itoa(id))
	if got.Summary != "" || got.Description != "changed" {
		t.Fatalf("after summary clear = %+v", got)
	}

	// Without --force and empty stdin the delete is cancelled.
	var cancelled actionEnvelope
	r.runJSON(t, &cancelled, "news", "delete", strconv.Itoa(id))
	if cancelled.Ok || cancelled.Action != "deleted" {
		t.Fatalf("unconfirmed delete envelope = %+v, want ok=false", cancelled)
	}
	if !containsInt(listNewsIDs(t, r, "--project", proj.Identifier), id) {
		t.Fatal("news deleted despite declined confirmation")
	}

	var del actionEnvelope
	r.runJSON(t, &del, "news", "delete", strconv.Itoa(id), "--force")
	if !del.Ok || del.Action != "deleted" || del.Resource != "news" || envelopeIntID(del.ID) != id {
		t.Fatalf("news delete envelope = %+v", del)
	}
	assertErrorCode(t, r.runExpectJSONError(t, "news", "get", strconv.Itoa(id)), "not_found")
	if containsInt(listNewsIDs(t, r, "--project", proj.Identifier), id) {
		t.Fatal("deleted news still listed")
	}
}

func TestNews_CreateWithAttachmentAndDescriptionFile(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	dir := t.TempDir()
	descFile := filepath.Join(dir, "body.txt")
	attachFile := filepath.Join(dir, "news-attachment.txt")
	if err := os.WriteFile(descFile, []byte("body from a file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(attachFile, []byte("attached to news"), 0o644); err != nil {
		t.Fatal(err)
	}

	id := createNewsCLI(t, r, proj.Identifier, "E2E news with file", "--description-file", descFile, "--attach", attachFile)

	var got newsJSON
	r.runJSON(t, &got, "news", "get", strconv.Itoa(id), "--include", "attachments")
	if got.Description != "body from a file" {
		t.Fatalf("description = %q", got.Description)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].Filename != "news-attachment.txt" {
		t.Fatalf("attachments = %+v", got.Attachments)
	}

	// Attach to an existing item through update.
	second := filepath.Join(dir, "second.txt")
	if err := os.WriteFile(second, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	r.run(t, "news", "update", strconv.Itoa(id), "--attach", second)
	got = newsJSON{}
	r.runJSON(t, &got, "news", "get", strconv.Itoa(id), "--include", "attachments")
	if len(got.Attachments) != 2 {
		t.Fatalf("attachments after update = %+v, want 2", got.Attachments)
	}
}

func TestNews_Validation(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	// Missing description is rejected before any request.
	_, stderr := r.runExpectError(t, "news", "create", "--project", proj.Identifier, "--title", "No description")
	if !strings.Contains(string(stderr), "description") {
		t.Errorf("stderr = %s, want a hint about --description", stderr)
	}
	if got := listNewsIDs(t, r, "--project", proj.Identifier); len(got) != 0 {
		t.Errorf("news created despite the missing description: %v", got)
	}

	// Redmine rejects titles over 60 characters.
	long := strings.Repeat("x", 61)
	assertErrorCode(t, r.runExpectJSONError(t, "news", "create", "--project", proj.Identifier,
		"--title", long, "--description", "d"), "validation_failed")

	// Nothing to change.
	_, stderr = r.runExpectError(t, "news", "update", "1")
	if !strings.Contains(string(stderr), "nothing to change") {
		t.Errorf("stderr = %s", stderr)
	}

	assertErrorCode(t, r.runExpectJSONError(t, "news", "get", "999999999"), "not_found")
	assertErrorCode(t, r.runExpectJSONError(t, "news", "delete", "999999999", "--force"), "not_found")
}

func TestNews_GetIncludesComments(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	w := loadContractWorld(t)
	id := strconv.Itoa(w.id("news"))

	var got newsJSON
	r.runJSON(t, &got, "news", "get", id, "--include", "comments,attachments")
	if got.Title != "Contract news" || got.Summary != "contract news summary" {
		t.Fatalf("news get = %+v", got)
	}
	if len(got.Comments) != 1 || got.Comments[0].Content != "contract news comment" {
		t.Fatalf("comments = %+v", got.Comments)
	}
	if len(got.Attachments) != 1 || got.Attachments[0].Filename != "contract-news.txt" {
		t.Fatalf("attachments = %+v", got.Attachments)
	}

	// Table output prints the comment and attachment sections.
	out := string(r.run(t, "news", "get", id, "--include", "comments,attachments", "-o", "table"))
	for _, want := range []string{"Contract news", "Comments:", "contract news comment", "Attachments:", "contract-news.txt"} {
		if !strings.Contains(out, want) {
			t.Errorf("table output missing %q:\n%s", want, out)
		}
	}

	// Without --include neither section is present.
	got = newsJSON{}
	r.runJSON(t, &got, "news", "get", id)
	if got.Comments != nil || got.Attachments != nil {
		t.Errorf("plain get leaked includes: %+v", got)
	}

	_, stderr := r.runExpectError(t, "news", "get", id, "--include", "journals")
	if !strings.Contains(string(stderr), "journals") {
		t.Errorf("stderr = %s, want the unknown include named", stderr)
	}
}

func TestNews_ListPagination(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	first := createNewsCLI(t, r, proj.Identifier, "E2E page one", "--description", "d")
	second := createNewsCLI(t, r, proj.Identifier, "E2E page two", "--description", "d")

	// Newest first.
	if got := listNewsIDs(t, r, "--project", proj.Identifier); len(got) != 2 || got[0] != second || got[1] != first {
		t.Fatalf("list = %v, want [%d %d]", got, second, first)
	}
	if got := listNewsIDs(t, r, "--project", proj.Identifier, "--limit", "1"); len(got) != 1 || got[0] != second {
		t.Fatalf("limit 1 = %v, want [%d]", got, second)
	}
	if got := listNewsIDs(t, r, "--project", proj.Identifier, "--limit", "1", "--offset", "1"); len(got) != 1 || got[0] != first {
		t.Fatalf("limit 1 offset 1 = %v, want [%d]", got, first)
	}

	_, stderr, err := r.runRaw("news", "list", "--project", proj.Identifier, "--limit", "1", "-o", "table")
	if err != nil {
		t.Fatalf("table list: %v", err)
	}
	if !strings.Contains(string(stderr), "--offset") {
		t.Errorf("stderr = %q, want a pagination warning", stderr)
	}
}

func TestMCPStdio_NewsTools(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	w := loadContractWorld(t)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
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

	// Reads against the contract world.
	list := mcpCallAny(t, ctx, session, "list_news", map[string]any{"project_id": contractProject})
	var ids []int
	listed, _ := field(list, "news").([]any)
	for _, it := range listed {
		ids = append(ids, jsonInt(field(it, "id")))
	}
	if !containsInt(ids, w.id("news")) {
		t.Fatalf("list_news ids %v miss %d", ids, w.id("news"))
	}
	one := mcpCallAny(t, ctx, session, "get_news", map[string]any{"id": w.id("news"), "includes": []string{"comments", "attachments"}})
	if title, _ := field(one, "title").(string); title != "Contract news" {
		t.Fatalf("get_news title = %v", field(one, "title"))
	}
	if cs, _ := field(one, "comments").([]any); len(cs) != 1 {
		t.Fatalf("get_news comments = %v", field(one, "comments"))
	}

	// Invalid input is rejected as a tool error before Redmine is asked.
	if out := call("get_news", map[string]any{"id": 0}); !out.IsError || !strings.Contains(structuredJSON(out), "positive news ID") {
		t.Fatalf("get_news id 0 should be a validation tool error: %s", structuredJSON(out))
	}
	if out := call("get_news", map[string]any{"id": w.id("news"), "includes": []string{"bogus"}}); !out.IsError || !strings.Contains(structuredJSON(out), "unknown include") {
		t.Fatalf("get_news with unknown include should be a validation tool error: %s", structuredJSON(out))
	}

	// Writes: create (with the read-back id), update, delete.
	created := mustOK("create_news", map[string]any{
		"project_id": proj.Identifier, "title": "E2E MCP news", "summary": "mcp summary", "description": "mcp description",
	})
	v, ok := mcpExtFieldFromResult(t, created, "id")
	f, isNum := v.(float64)
	if !ok || !isNum || f == 0 {
		t.Fatalf("create_news returned no id: %s", structuredJSON(created))
	}
	id := int(f)

	var got newsJSON
	r.runJSON(t, &got, "news", "get", strconv.Itoa(id))
	if got.Title != "E2E MCP news" || got.Summary != "mcp summary" {
		t.Fatalf("news after create_news = %+v", got)
	}

	mustOK("update_news", map[string]any{"id": id, "title": "E2E MCP news v2", "description": "mcp changed"})
	got = newsJSON{}
	r.runJSON(t, &got, "news", "get", strconv.Itoa(id))
	if got.Title != "E2E MCP news v2" || got.Description != "mcp changed" || got.Summary != "mcp summary" {
		t.Fatalf("news after update_news = %+v", got)
	}

	if out := call("update_news", map[string]any{"id": id}); !out.IsError {
		t.Fatalf("update_news without fields should be an error: %s", structuredJSON(out))
	}
	if out := call("create_news", map[string]any{"project_id": proj.Identifier, "title": "no description"}); !out.IsError {
		t.Fatalf("create_news without description should be an error: %s", structuredJSON(out))
	}

	mustOK("delete_news", map[string]any{"id": id})
	assertErrorCode(t, r.runExpectJSONError(t, "news", "get", strconv.Itoa(id)), "not_found")

	t.Run("groups", func(t *testing.T) {
		newsOnly := r.startMCPStdio(t, ctx, "--enable-groups", "news", "--enable-writes")
		tools, err := newsOnly.ListTools(ctx, nil)
		if err != nil {
			t.Fatalf("ListTools: %v", err)
		}
		names := toolNames(tools.Tools)
		want := []string{"list_news", "get_news", "create_news", "update_news", "delete_news"}
		mustContain(t, names, want...)
		for _, n := range names {
			if !strings.HasSuffix(n, "_news") {
				t.Errorf("--enable-groups news leaked %q", n)
			}
		}
	})
}
