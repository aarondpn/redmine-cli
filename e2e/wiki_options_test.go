//go:build e2e

package e2e

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

type wikiListEntry struct {
	Title  string `json:"title"`
	Parent *struct {
		Title string `json:"title"`
	} `json:"parent"`
}

func listWiki(t *testing.T, r *cliRunner, project string) map[string]wikiListEntry {
	t.Helper()
	var pages []wikiListEntry
	r.runJSON(t, &pages, "wiki", "list", "--project", project)
	out := make(map[string]wikiListEntry, len(pages))
	for _, p := range pages {
		out[p.Title] = p
	}
	return out
}

func (e wikiListEntry) parent() string {
	if e.Parent == nil {
		return ""
	}
	return e.Parent.Title
}

// createWikiTree creates <prefix>_Parent with two children, one of which has
// a grandchild, plus an unrelated <prefix>_Target page.
func createWikiTree(t *testing.T, r *cliRunner, project string) (parent, childA, childB, grandchild, target string) {
	t.Helper()
	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	parent, childA, childB = "Parent_"+suffix, "ChildA_"+suffix, "ChildB_"+suffix
	grandchild, target = "Grand_"+suffix, "Target_"+suffix
	for _, p := range []struct{ page, parent string }{
		{parent, ""}, {target, ""}, {childA, parent}, {childB, parent}, {grandchild, childB},
	} {
		args := []string{"wiki", "create", p.page, "--project", project, "--text", "Body of " + p.page}
		if p.parent != "" {
			args = append(args, "--parent", p.parent)
		}
		r.run(t, args...)
	}
	return
}

// TestWiki_DeleteChildren covers --children. Both outcomes behave the same on every supported Redmine version, so no gate.
func TestWiki_DeleteChildren(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	t.Run("nullify", func(t *testing.T) {
		parent, childA, childB, grandchild, _ := createWikiTree(t, r, proj.Identifier)
		r.run(t, "wiki", "delete", parent, "--project", proj.Identifier, "--children", "nullify", "--force")
		pages := listWiki(t, r, proj.Identifier)
		if _, ok := pages[parent]; ok {
			t.Fatalf("%s still exists", parent)
		}
		for _, c := range []string{childA, childB} {
			if p, ok := pages[c]; !ok || p.parent() != "" {
				t.Errorf("%s = %+v (exists=%v), want a root page", c, p, ok)
			}
		}
		if pages[grandchild].parent() != childB {
			t.Errorf("grandchild parent = %q, want %q", pages[grandchild].parent(), childB)
		}
	})

	t.Run("destroy", func(t *testing.T) {
		parent, childA, childB, grandchild, target := createWikiTree(t, r, proj.Identifier)
		r.run(t, "wiki", "delete", parent, "--project", proj.Identifier, "--children", "destroy", "--force")
		pages := listWiki(t, r, proj.Identifier)
		for _, gone := range []string{parent, childA, childB, grandchild} {
			if _, ok := pages[gone]; ok {
				t.Errorf("%s still exists after destroy", gone)
			}
		}
		if _, ok := pages[target]; !ok {
			t.Errorf("unrelated page %s was deleted", target)
		}
	})

	t.Run("reassign is not offered", func(t *testing.T) {
		parent, _, _, _, target := createWikiTree(t, r, proj.Identifier)
		r.runExpectError(t, "wiki", "delete", parent, "--project", proj.Identifier, "--children", "reassign", "--force")
		if _, ok := listWiki(t, r, proj.Identifier)[parent]; !ok {
			t.Error("page was deleted by a rejected request")
		}
		_ = target
	})
}

// TestWiki_UpdateOptions covers --start-page, --redirect-existing-links and
// --remove-attachment against the real API.
func TestWiki_UpdateOptions(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	base := "/projects/" + url.PathEscape(proj.Identifier)

	t.Run("start page", func(t *testing.T) {
		page := wikiPageName(t)
		r.run(t, "wiki", "create", page, "--project", proj.Identifier, "--text", "start here")
		r.run(t, "wiki", "update", page, "--project", proj.Identifier, "--start-page")
		var start struct {
			WikiPage struct {
				Title string `json:"title"`
			} `json:"wiki_page"`
		}
		// Without a page title, Redmine serves the wiki's start page.
		r.runJSON(t, &start, "api", base+"/wiki.json")
		if start.WikiPage.Title != page {
			t.Errorf("start page = %q, want %q", start.WikiPage.Title, page)
		}
	})

	t.Run("rename without redirect", func(t *testing.T) {
		page := wikiPageName(t)
		renamed := page + "-renamed"
		r.run(t, "wiki", "create", page, "--project", proj.Identifier, "--text", "body")
		r.run(t, "wiki", "update", page, "--project", proj.Identifier,
			"--title", renamed, "--redirect-existing-links=false")
		r.run(t, "wiki", "get", renamed, "--project", proj.Identifier)
		// No redirect was left, so the old title is simply gone.
		stderr := r.runExpectJSONError(t, "wiki", "get", page, "--project", proj.Identifier)
		assertErrorCode(t, stderr, "not_found")
	})

	t.Run("redirect flag needs a rename", func(t *testing.T) {
		page := wikiPageName(t)
		r.run(t, "wiki", "create", page, "--project", proj.Identifier, "--text", "body")
		r.runExpectError(t, "wiki", "update", page, "--project", proj.Identifier,
			"--redirect-existing-links=false")
	})

	t.Run("remove attachment", func(t *testing.T) {
		dir := t.TempDir()
		for _, name := range []string{"keep.txt", "drop.txt"} {
			if err := os.WriteFile(filepath.Join(dir, name), []byte(name), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		page := wikiPageName(t)
		r.run(t, "wiki", "create", page, "--project", proj.Identifier, "--text", "files",
			"--attach", filepath.Join(dir, "keep.txt"), "--attach", filepath.Join(dir, "drop.txt"))

		type attachments struct {
			Attachments []struct {
				ID       int    `json:"id"`
				Filename string `json:"filename"`
			} `json:"attachments"`
		}
		var before attachments
		r.runJSON(t, &before, "wiki", "get", page, "--project", proj.Identifier, "--include", "attachments")
		dropID := 0
		for _, a := range before.Attachments {
			if a.Filename == "drop.txt" {
				dropID = a.ID
			}
		}
		if dropID == 0 || len(before.Attachments) != 2 {
			t.Fatalf("attachments before = %+v", before.Attachments)
		}

		r.run(t, "wiki", "update", page, "--project", proj.Identifier, "--remove-attachment", strconv.Itoa(dropID), "--force")

		var after attachments
		r.runJSON(t, &after, "wiki", "get", page, "--project", proj.Identifier, "--include", "attachments")
		if len(after.Attachments) != 1 || after.Attachments[0].Filename != "keep.txt" {
			t.Errorf("attachments after = %+v, want only keep.txt", after.Attachments)
		}
	})
}

// TestMCP_WikiOptions covers get_wiki_page version, delete_wiki_page todo, update_wiki_page start page and the search attachments filter.
func TestMCP_WikiOptions(t *testing.T) {
	requireE2E(t)
	for name, start := range mcpTransports {
		t.Run(name, func(t *testing.T) {
			r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
			proj := createTestProject(t, r)
			ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
			defer cancel()
			session := start(t, ctx, r)

			page := wikiPageName(t)
			mcpCallOK(t, ctx, session, "create_wiki_page", map[string]any{
				"project_id": proj.Identifier, "page": page, "text": "first",
			})
			mcpCallOK(t, ctx, session, "update_wiki_page", map[string]any{
				"project_id": proj.Identifier, "page": page, "text": "second", "is_start_page": true,
			})
			old := mcpCallOK(t, ctx, session, "get_wiki_page", map[string]any{
				"project_id": proj.Identifier, "page": page, "version": 1,
			})
			if text, _ := mcpExtFieldFromResult(t, old, "text"); text != "first" {
				t.Errorf("version 1 text = %v, want first", text)
			}

			parent, childA, _, grandchild, _ := createWikiTree(t, r, proj.Identifier)
			mcpCallOK(t, ctx, session, "delete_wiki_page", map[string]any{
				"project_id": proj.Identifier, "page": parent, "todo": "destroy",
			})
			pages := listWiki(t, r, proj.Identifier)
			for _, gone := range []string{parent, childA, grandchild} {
				if _, ok := pages[gone]; ok {
					t.Errorf("%s still exists after destroy", gone)
				}
			}

			mcpCallOK(t, ctx, session, "search", map[string]any{
				"query": "second", "project_id": proj.Identifier, "attachments": "0",
			})
			if out := mcpCall(t, ctx, session, "search", map[string]any{"query": "x", "attachments": "bogus"}); !out.IsError {
				t.Error("search with an invalid attachments value must fail")
			}
		})
	}
}
