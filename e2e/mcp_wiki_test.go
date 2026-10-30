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

// mcpTransports starts a write-enabled MCP session for r over each transport,
// so tool behavior is checked over stdio and HTTP alike.
var mcpTransports = map[string]func(t *testing.T, ctx context.Context, r *cliRunner) *mcp.ClientSession{
	"stdio": func(t *testing.T, ctx context.Context, r *cliRunner) *mcp.ClientSession {
		return r.startMCPStdio(t, ctx, "--enable-writes")
	},
	"http": func(t *testing.T, ctx context.Context, r *cliRunner) *mcp.ClientSession {
		session, _ := r.startMCPHTTP(t, ctx, "", "--enable-writes")
		return session
	},
}

// mcpCall invokes a tool and returns its result, failing on protocol errors
// only; tool errors are reported through IsError for the caller to check.
func mcpCall(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	out, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool %s: %v", name, err)
	}
	return out
}

func mcpCallOK(t *testing.T, ctx context.Context, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	out := mcpCall(t, ctx, session, name, args)
	if out.IsError {
		t.Fatalf("%s returned IsError: %s", name, structuredJSON(out))
	}
	return out
}

// mcpWikiParent returns the parent title of page as reported by get_wiki_page
// ("" for the wiki root).
func mcpWikiParent(t *testing.T, ctx context.Context, session *mcp.ClientSession, project, page string) string {
	t.Helper()
	out := mcpCallOK(t, ctx, session, "get_wiki_page", map[string]any{"project_id": project, "page": page})
	parent, ok := mcpExtFieldFromResult(t, out, "parent")
	if !ok || parent == nil {
		return ""
	}
	title, _ := parent.(map[string]any)["title"].(string)
	return title
}

// TestMCP_WikiParent covers parent_title on create_wiki_page/update_wiki_page
// and the parent field on list_wiki_pages, over both transports.
func TestMCP_WikiParent(t *testing.T) {
	requireE2E(t)
	for name, start := range mcpTransports {
		t.Run(name, func(t *testing.T) {
			r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
			proj := createTestProject(t, r)

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			session := start(t, ctx, r)

			tools, err := session.ListTools(ctx, nil)
			if err != nil {
				t.Fatalf("ListTools: %v", err)
			}
			for _, tool := range tools.Tools {
				if tool.Name != "create_wiki_page" && tool.Name != "update_wiki_page" {
					continue
				}
				schema, _ := json.Marshal(tool.InputSchema)
				if !strings.Contains(string(schema), `"parent_title"`) {
					t.Errorf("%s input schema has no parent_title: %s", tool.Name, schema)
				}
			}

			suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
			guide, other, child := "Guide_"+suffix, "Other_"+suffix, "Child_"+suffix
			for _, page := range []string{guide, other} {
				mcpCallOK(t, ctx, session, "create_wiki_page", map[string]any{
					"project_id": proj.Identifier, "page": page, "text": "Parent page.",
				})
			}

			created := mcpCallOK(t, ctx, session, "create_wiki_page", map[string]any{
				"project_id": proj.Identifier, "page": child, "text": "Child page.",
				"parent_title": "guide " + suffix,
			})
			if parent, _ := mcpExtFieldFromResult(t, created, "parent"); parent == nil {
				t.Errorf("create_wiki_page result has no parent: %s", structuredJSON(created))
			}
			if got := mcpWikiParent(t, ctx, session, proj.Identifier, child); got != guide {
				t.Fatalf("child parent after create = %q, want %q", got, guide)
			}

			listed := mcpCallOK(t, ctx, session, "list_wiki_pages", map[string]any{"project_id": proj.Identifier})
			rawPages, _ := mcpExtFieldFromResult(t, listed, "pages")
			pages, ok := rawPages.([]any)
			if !ok {
				t.Fatalf("list_wiki_pages returned no pages array: %s", structuredJSON(listed))
			}
			foundChild := false
			for _, p := range pages {
				entry, _ := p.(map[string]any)
				if entry["title"] != child {
					continue
				}
				foundChild = true
				parent, _ := entry["parent"].(map[string]any)
				if parent == nil || parent["title"] != guide {
					t.Errorf("list_wiki_pages: child parent = %v, want %q", entry["parent"], guide)
				}
			}
			if !foundChild {
				t.Fatalf("list_wiki_pages did not include %q: %s", child, structuredJSON(listed))
			}

			// A move without text: Redmine requires text on every update, so
			// the tool resends the current body.
			mcpCallOK(t, ctx, session, "update_wiki_page", map[string]any{
				"project_id": proj.Identifier, "page": child, "parent_title": other,
			})
			if got := mcpWikiParent(t, ctx, session, proj.Identifier, child); got != other {
				t.Fatalf("child parent after move = %q, want %q", got, other)
			}
			moved := mcpCallOK(t, ctx, session, "get_wiki_page", map[string]any{"project_id": proj.Identifier, "page": child})
			if text, _ := mcpExtFieldFromResult(t, moved, "text"); text != "Child page." {
				t.Errorf("text after move without text = %v, want it preserved", text)
			}

			mcpCallOK(t, ctx, session, "update_wiki_page", map[string]any{
				"project_id": proj.Identifier, "page": child, "text": "Child page.", "parent_title": "",
			})
			if got := mcpWikiParent(t, ctx, session, proj.Identifier, child); got != "" {
				t.Fatalf("child parent after clear = %q, want the wiki root", got)
			}

			bad := mcpCall(t, ctx, session, "update_wiki_page", map[string]any{
				"project_id": proj.Identifier, "page": child, "text": "Child page.", "parent_title": "Missing_" + suffix,
			})
			if !bad.IsError {
				t.Fatalf("update_wiki_page with unknown parent succeeded: %s", structuredJSON(bad))
			}
			if text := mcpExtTextContent(t, bad); !strings.Contains(text, "Missing_"+suffix) {
				t.Errorf("unknown parent error does not name the parent: %q", text)
			}
		})
	}
}

// TestMCP_WikiParentNotApplied_NonAdmin runs the MCP server as a Developer
// (no rename_wiki_pages): moving an existing page must come back as a tool
// error instead of a silent success.
func TestMCP_WikiParentNotApplied_NonAdmin(t *testing.T) {
	requireE2E(t)
	for name, start := range mcpTransports {
		t.Run(name, func(t *testing.T) {
			admin := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
			proj := createTestProject(t, admin)
			member := createTestUser(t, admin)
			addProjectMember(t, admin, proj.Identifier, "--user-id", member.ID, roleIDByName(t, admin, "Developer"))
			r := newCLIRunnerBasicAuth(t, e2eBaseURL(), member.Login, member.Password)

			suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
			guide, page := "Guide_"+suffix, "Page_"+suffix
			admin.run(t, "wiki", "create", guide, "--project", proj.Identifier, "--text", "Guide.")
			admin.run(t, "wiki", "create", page, "--project", proj.Identifier, "--text", "Body.")

			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			session := start(t, ctx, r)

			out := mcpCall(t, ctx, session, "update_wiki_page", map[string]any{
				"project_id": proj.Identifier, "page": page, "parent_title": guide,
			})
			if !out.IsError {
				t.Fatalf("update_wiki_page reported success although Redmine dropped the parent: %s", structuredJSON(out))
			}
			text := mcpExtTextContent(t, out)
			for _, want := range []string{"parent was not changed", guide, "rename_wiki_pages"} {
				if !strings.Contains(text, want) {
					t.Errorf("tool error missing %q: %q", want, text)
				}
			}
			// Only the parent was requested; the resent text is unchanged.
			if strings.Contains(text, "may already have been applied") {
				t.Errorf("parent-only call claims other changes: %q", text)
			}
			requireWikiParent(t, admin, proj.Identifier, page, "")
		})
	}
}
