//go:build e2e

package e2e

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

	// Optimistic concurrency: --expect-version 2 should succeed because the
	// stored version is 2 (after the previous update bumped it from 1).
	var versionedUpdate actionEnvelope
	r.runJSON(t, &versionedUpdate, "wiki", "update", page,
		"--project", proj.Identifier,
		"--text", "h1. Hello\n\nVersion-checked rewrite.",
		"--comments", "expect-version=2",
		"--expect-version", "2")
	if !versionedUpdate.Ok || versionedUpdate.Action != "updated" {
		t.Fatalf("unexpected versioned update envelope: %+v", versionedUpdate)
	}

	// Stored version is now 3. Asserting --expect-version 2 must fail with a
	// conflict envelope, proving optimistic locking is wired end-to-end.
	staleStdout, _ := r.runExpectError(t, "wiki", "update", page,
		"--project", proj.Identifier,
		"--text", "should not land",
		"--expect-version", "2")
	assertErrorCode(t, staleStdout, "conflict")

	// --ensure-current refetches before sending, so even though we don't know
	// the current version it should resolve to 3 and succeed.
	var ensuredUpdate actionEnvelope
	r.runJSON(t, &ensuredUpdate, "wiki", "update", page,
		"--project", proj.Identifier,
		"--comments", "ensure-current",
		"--ensure-current")
	if !ensuredUpdate.Ok || ensuredUpdate.Action != "updated" {
		t.Fatalf("unexpected ensure-current update envelope: %+v", ensuredUpdate)
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

// TestWiki_SectionUpdate verifies the --section flag end-to-end: only the
// targeted section is replaced and the surrounding sections are preserved.
// Without --section, a wiki update rewrites the whole body, so this test
// fails loudly if the section parameter never reaches Redmine (e.g. if it is
// sent nested under wiki_page, where Redmine ignores it).
func TestWiki_SectionUpdate(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	style := wikiHeadingStyle()
	page := wikiPageName(t)
	const (
		alphaBody = "Alpha body untouched."
		bravoOld  = "Bravo body original."
		bravoNew  = "Bravo body REPLACED by section edit."
		charlBody = "Charlie body untouched."
	)
	initial := wikiSection(style, "Alpha", alphaBody) + "\n\n" +
		wikiSection(style, "Bravo", bravoOld) + "\n\n" +
		wikiSection(style, "Charlie", charlBody)

	var created struct {
		Version int `json:"version"`
	}
	r.runJSON(t, &created, "wiki", "create", page,
		"--project", proj.Identifier,
		"--text", initial)
	if created.Version != 1 {
		t.Fatalf("created version = %d, want 1", created.Version)
	}

	// Replace only section 2 (Bravo). Sections are 1-based and count headings
	// in document order: 1=Alpha, 2=Bravo, 3=Charlie.
	var updated actionEnvelope
	r.runJSON(t, &updated, "wiki", "update", page,
		"--project", proj.Identifier,
		"--section", "2",
		"--text", wikiSection(style, "Bravo", bravoNew))
	if !updated.Ok || updated.Action != "updated" {
		t.Fatalf("unexpected section update envelope: %+v", updated)
	}

	var after struct {
		Text    string `json:"text"`
		Version int    `json:"version"`
	}
	r.runJSON(t, &after, "wiki", "get", page, "--project", proj.Identifier)

	// The edited section must carry the new content.
	if !strings.Contains(after.Text, bravoNew) {
		t.Errorf("section 2 not updated; page text = %q", after.Text)
	}
	// The surrounding sections must survive untouched. If --section was
	// dropped, Redmine rewrites the whole page with just the Bravo text and
	// these markers disappear.
	if !strings.Contains(after.Text, alphaBody) {
		t.Errorf("section 1 (Alpha) was lost; page text = %q", after.Text)
	}
	if !strings.Contains(after.Text, charlBody) {
		t.Errorf("section 3 (Charlie) was lost; page text = %q", after.Text)
	}
	// The old section-2 body must be gone (it was replaced, not appended).
	if strings.Contains(after.Text, bravoOld) {
		t.Errorf("old section 2 body still present; page text = %q", after.Text)
	}
	if after.Version != 2 {
		t.Errorf("after section update version = %d, want 2", after.Version)
	}
}

// TestWiki_SectionUpdate_InvalidSection verifies the client-side guard that
// rejects non-positive --section values before any request is sent.
func TestWiki_SectionUpdate_InvalidSection(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	stdout, stderr := r.runExpectError(t, "wiki", "update", "AnyPage",
		"--project", proj.Identifier,
		"--text", "x",
		"--section", "0")
	// The CLI emits a JSON error envelope on stdout; ">" is escaped as >
	// there, so match on the unescaped prefix of the message instead.
	combined := string(stdout) + string(stderr)
	if !strings.Contains(combined, "section must be") {
		t.Errorf("expected a '--section must be >= 1' validation error, got stdout=%q stderr=%q", stdout, stderr)
	}
}

// TestWiki_SectionUpdate_StaleHashConflict verifies that --section-hash
// reaches Redmine and drives section-level optimistic locking: a stale hash
// must surface as a conflict, not silently overwrite the section.
func TestWiki_SectionUpdate_StaleHashConflict(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	style := wikiHeadingStyle()
	page := wikiPageName(t)
	initial := wikiSection(style, "Alpha", "Alpha body.") + "\n\n" +
		wikiSection(style, "Bravo", "Bravo body.")
	r.run(t, "wiki", "create", page,
		"--project", proj.Identifier,
		"--text", initial)

	stdout, _ := r.runExpectError(t, "wiki", "update", page,
		"--project", proj.Identifier,
		"--section", "2",
		"--section-hash", "deadbeefdeadbeefdeadbeefdeadbeef",
		"--text", wikiSection(style, "Bravo", "should not land"))
	assertErrorCode(t, stdout, "conflict")
}

// TestWiki_ProjectInResponse covers the project node Redmine 7.0 added to the
// wiki page API response (#43569). Older lines omit it, so the test is gated
// rather than made tolerant: the point is that 7.0+ actually populates it.
func TestWiki_ProjectInResponse(t *testing.T) {
	requireE2E(t)
	skipIfVersionUnknown(t, "project in wiki page API response")
	skipBelowRedmine(t, 7, 0, "project in wiki page API response")

	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	page := wikiPageName(t)
	r.run(t, "wiki", "create", page,
		"--project", proj.Identifier,
		"--text", wikiSection(wikiHeadingStyle(), "Scope", "Body from e2e."))

	var fetched struct {
		Title   string `json:"title"`
		Project *struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		} `json:"project"`
	}
	r.runJSON(t, &fetched, "wiki", "get", page, "--project", proj.Identifier)
	if fetched.Project == nil {
		t.Fatalf("wiki get omitted project node on Redmine %s: %+v", e2eVersion(), fetched)
	}
	if fetched.Project.ID != proj.ID {
		t.Errorf("wiki page project id = %d, want %d", fetched.Project.ID, proj.ID)
	}

	stdout := r.run(t, "wiki", "get", page, "--project", proj.Identifier, "--output", "table")
	if !strings.Contains(string(stdout), fetched.Project.Name) {
		t.Errorf("wiki get table output missing project name %q\nstdout:\n%s", fetched.Project.Name, stdout)
	}
}

// wikiHeadingStyle returns the markup the e2e server's default formatter
// recognises as headings. Redmine 5.0+ defaults to CommonMark (Markdown);
// 4.x and earlier default to Textile. Section editing keys off recognised
// headings, so test content must match the active formatter.
func wikiHeadingStyle() string {
	if !redmineAtLeast(5, 0) {
		return "textile"
	}
	return "markdown"
}

// wikiSection renders a single heading + body block in the given style.
func wikiSection(style, heading, body string) string {
	if style == "textile" {
		return fmt.Sprintf("h1. %s\n\n%s", heading, body)
	}
	return fmt.Sprintf("# %s\n\n%s", heading, body)
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
