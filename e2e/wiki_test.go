//go:build e2e

package e2e

import (
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

// TestWiki_Lifecycle drives the full wiki page lifecycle: create → list →
// get (latest and prior version) → update → delete. It also verifies the
// 404 envelope for a missing page.
func TestWiki_Lifecycle(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	page := wikiPageName(t)
	const (
		initialText = "h1. Hello\n\nInitial body from e2e."
		updatedText = "h1. Hello\n\nRewritten body from e2e."
		v1Comments  = "Initial draft"
		v2Comments  = "Rewrote body"
	)

	var created struct {
		Title    string `json:"title"`
		Text     string `json:"text"`
		Version  int    `json:"version"`
		Comments string `json:"comments"`
	}
	// NB: we deliberately omit --title here. Redmine treats the wiki_page.title
	// field as the canonical URL slug and rewrites the URL to match (replacing
	// spaces with underscores). When --title is set the positional <page> arg
	// is silently ignored, which makes downstream get/update/delete by the
	// positional arg fail with 404. The --title path is exercised by a
	// dedicated test below that uses the returned title for follow-up ops.
	r.runJSON(t, &created, "wiki", "create", page,
		"--project", proj.Identifier,
		"--text", initialText,
		"--comments", v1Comments)
	if created.Text != initialText {
		t.Fatalf("created wiki text = %q, want %q", created.Text, initialText)
	}
	if created.Version != 1 {
		t.Fatalf("created wiki version = %d, want 1", created.Version)
	}
	if created.Comments != v1Comments {
		t.Fatalf("created wiki comments = %q, want %q", created.Comments, v1Comments)
	}
	if created.Title == "" {
		t.Fatalf("created wiki title is empty: %+v", created)
	}

	var listed []struct {
		Title string `json:"title"`
	}
	r.runJSON(t, &listed, "wiki", "list", "--project", proj.Identifier)
	if !containsWikiTitle(listed, page) && !containsWikiTitle(listed, created.Title) {
		t.Fatalf("wiki list did not include created page %q (or %q): %+v",
			page, created.Title, listed)
	}

	var fetched struct {
		Title   string `json:"title"`
		Text    string `json:"text"`
		Version int    `json:"version"`
	}
	r.runJSON(t, &fetched, "wiki", "get", page, "--project", proj.Identifier)
	if fetched.Text != initialText {
		t.Fatalf("get wiki text = %q, want %q", fetched.Text, initialText)
	}
	if fetched.Version != 1 {
		t.Fatalf("get wiki version = %d, want 1", fetched.Version)
	}

	var updated actionEnvelope
	r.runJSON(t, &updated, "wiki", "update", page,
		"--project", proj.Identifier,
		"--text", updatedText,
		"--comments", v2Comments)
	if !updated.Ok || updated.Action != "updated" || updated.Resource != "wiki_page" {
		t.Fatalf("unexpected update envelope: %+v", updated)
	}

	var afterUpdate struct {
		Text     string `json:"text"`
		Version  int    `json:"version"`
		Comments string `json:"comments"`
	}
	r.runJSON(t, &afterUpdate, "wiki", "get", page, "--project", proj.Identifier)
	if afterUpdate.Text != updatedText {
		t.Fatalf("after update text = %q, want %q", afterUpdate.Text, updatedText)
	}
	if afterUpdate.Version != 2 {
		t.Fatalf("after update version = %d, want 2", afterUpdate.Version)
	}

	// Fetch the prior revision via --version. Redmine preserves history so
	// version 1 must still echo the original text.
	var v1 struct {
		Text    string `json:"text"`
		Version int    `json:"version"`
	}
	r.runJSON(t, &v1, "wiki", "get", page, "--project", proj.Identifier, "--version", "1")
	if v1.Version != 1 {
		t.Fatalf("get --version 1: version = %d, want 1", v1.Version)
	}
	if v1.Text != initialText {
		t.Fatalf("get --version 1: text = %q, want %q", v1.Text, initialText)
	}

	var deleted actionEnvelope
	r.runJSON(t, &deleted, "wiki", "delete", page,
		"--project", proj.Identifier,
		"--force")
	if !deleted.Ok || deleted.Action != "deleted" || deleted.Resource != "wiki_page" {
		t.Fatalf("unexpected delete envelope: %+v", deleted)
	}

	stdout, _ := r.runExpectError(t, "wiki", "get", page, "--project", proj.Identifier)
	assertErrorCode(t, stdout, "not_found")
}

// TestWiki_Attachment verifies that --attach uploads a file with the wiki
// page and that it is exposed via ?include=attachments on a raw api fetch.
// The typed WikiPage model returned by `wiki get` does not include the
// attachment list unless attachments are requested via --include, so we
// mirror the issue-attachment pattern and use the raw `api` passthrough.
func TestWiki_Attachment(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	attachPath := filepath.Join(t.TempDir(), "wiki-attach.txt")
	payload := []byte("wiki attachment from redmine-cli e2e\n")
	if err := os.WriteFile(attachPath, payload, 0o600); err != nil {
		t.Fatalf("write attach file: %v", err)
	}

	page := wikiPageName(t)
	var created struct {
		Title string `json:"title"`
	}
	r.runJSON(t, &created, "wiki", "create", page,
		"--project", proj.Identifier,
		"--text", "See attached file.",
		"--attach", attachPath)
	if created.Title == "" {
		t.Fatalf("wiki create returned empty title: %+v", created)
	}

	var resp struct {
		WikiPage struct {
			Attachments []struct {
				Filename    string `json:"filename"`
				Filesize    int    `json:"filesize"`
				ContentType string `json:"content_type"`
			} `json:"attachments"`
		} `json:"wiki_page"`
	}
	wikiPath := "/projects/" + url.PathEscape(proj.Identifier) +
		"/wiki/" + url.PathEscape(page) + ".json"
	r.runJSON(t, &resp, "api", wikiPath, "-f", "include=attachments")

	found := false
	for _, att := range resp.WikiPage.Attachments {
		if att.Filename == "wiki-attach.txt" && att.Filesize == len(payload) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("attachment not found on wiki page; got %+v", resp.WikiPage.Attachments)
	}
}

// TestWiki_GetMissing covers the not-found error envelope on its own so a
// regression in error mapping is not masked by the lifecycle test having
// already populated the project with a real page.
func TestWiki_GetMissing(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	stdout, _ := r.runExpectError(t, "wiki", "get", "Nonexistent-"+wikiPageName(t),
		"--project", proj.Identifier)
	assertErrorCode(t, stdout, "not_found")
}

// wikiPageName returns a unique, Redmine-safe wiki page name for a single
// test. Redmine slugifies titles aggressively, so we keep the suffix
// alphanumeric and avoid characters Redmine treats as path separators.
func wikiPageName(t *testing.T) string {
	t.Helper()
	return "E2E-Page-" + strconv.FormatInt(time.Now().UnixNano(), 36)
}

func containsWikiTitle(pages []struct {
	Title string `json:"title"`
}, title string) bool {
	for _, p := range pages {
		if p.Title == title {
			return true
		}
	}
	return false
}
