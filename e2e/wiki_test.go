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
	staleStderr := r.runExpectJSONError(t, "wiki", "update", page,
		"--project", proj.Identifier,
		"--text", "should not land",
		"--expect-version", "2")
	assertErrorCode(t, staleStderr, "conflict")

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

	stderr := r.runExpectJSONError(t, "wiki", "get", page, "--project", proj.Identifier)
	assertErrorCode(t, stderr, "not_found")
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

	stderr := r.runExpectJSONError(t, "wiki", "get", "Nonexistent-"+wikiPageName(t),
		"--project", proj.Identifier)
	assertErrorCode(t, stderr, "not_found")
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
	// The CLI emits a JSON error envelope on stderr; ">" is escaped as >
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

	stderr := r.runExpectJSONError(t, "wiki", "update", page,
		"--project", proj.Identifier,
		"--section", "2",
		"--section-hash", "deadbeefdeadbeefdeadbeefdeadbeef",
		"--text", wikiSection(style, "Bravo", "should not land"))
	assertErrorCode(t, stderr, "conflict")
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

// wikiParent is the subset of a wiki page JSON payload the parent tests read.
type wikiParent struct {
	Title  string `json:"title"`
	Text   string `json:"text"`
	Parent *struct {
		Title string `json:"title"`
	} `json:"parent"`
}

func (p wikiParent) parentTitle() string {
	if p.Parent == nil {
		return ""
	}
	return p.Parent.Title
}

// requireWikiParent fetches page and fails unless its parent is want ("" for
// the wiki root).
func requireWikiParent(t *testing.T, r *cliRunner, project, page, want string) wikiParent {
	t.Helper()
	var got wikiParent
	r.runJSON(t, &got, "wiki", "get", page, "--project", project)
	if got.parentTitle() != want {
		t.Fatalf("wiki page %q parent = %q, want %q", page, got.parentTitle(), want)
	}
	return got
}

// TestWiki_Parent drives --parent end-to-end: create a child page, see the
// hierarchy in list, move the page, move it back to the root, and get a
// useful error for an unknown or circular parent.
func TestWiki_Parent(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	guide := "Guide_" + suffix
	other := "Other_" + suffix
	child := "Child_" + suffix
	style := wikiHeadingStyle()
	for _, page := range []string{guide, other} {
		r.run(t, "wiki", "create", page, "--project", proj.Identifier,
			"--text", wikiSection(style, page, "Parent page."))
	}

	// Redmine titleizes and matches case-insensitively: "guide <suffix>"
	// with a space must resolve to Guide_<suffix>.
	var created wikiParent
	r.runJSON(t, &created, "wiki", "create", child, "--project", proj.Identifier,
		"--text", wikiSection(style, "Child", "Child page."),
		"--parent", "guide "+suffix)
	if created.parentTitle() != guide {
		t.Fatalf("created child parent = %q, want %q", created.parentTitle(), guide)
	}
	requireWikiParent(t, r, proj.Identifier, child, guide)

	var listed []struct {
		Title   string `json:"title"`
		Version int    `json:"version"`
		Parent  *struct {
			Title string `json:"title"`
		} `json:"parent"`
	}
	r.runJSON(t, &listed, "wiki", "list", "--project", proj.Identifier)
	foundChild := false
	for _, p := range listed {
		switch p.Title {
		case child:
			foundChild = true
			if p.Parent == nil || p.Parent.Title != guide {
				t.Errorf("wiki list: child parent = %+v, want %q", p.Parent, guide)
			}
			if p.Version != 1 {
				t.Errorf("wiki list: child version = %d, want 1", p.Version)
			}
		case guide:
			if p.Parent != nil {
				t.Errorf("wiki list: top-level page %q has parent %+v", guide, p.Parent)
			}
		}
	}
	if !foundChild {
		t.Fatalf("wiki list did not include %q: %+v", child, listed)
	}
	table := string(r.run(t, "wiki", "list", "--project", proj.Identifier, "--output", "table"))
	if !strings.Contains(table, "Parent") {
		t.Errorf("wiki list table missing Parent column:\n%s", table)
	}

	// Move without --text: the current body must be preserved.
	var moved actionEnvelope
	r.runJSON(t, &moved, "wiki", "update", child, "--project", proj.Identifier, "--parent", other)
	if !moved.Ok || moved.Action != "updated" {
		t.Fatalf("unexpected move envelope: %+v", moved)
	}
	after := requireWikiParent(t, r, proj.Identifier, child, other)
	if !strings.Contains(after.Text, "Child page.") {
		t.Errorf("moving the page changed its text: %q", after.Text)
	}

	var cleared actionEnvelope
	r.runJSON(t, &cleared, "wiki", "update", child, "--project", proj.Identifier, "--parent", "")
	if !cleared.Ok {
		t.Fatalf("unexpected clear envelope: %+v", cleared)
	}
	requireWikiParent(t, r, proj.Identifier, child, "")

	// Redmine treats a whitespace-only parent like an empty one.
	r.run(t, "wiki", "update", child, "--project", proj.Identifier, "--parent", other)
	r.run(t, "wiki", "update", child, "--project", proj.Identifier, "--parent", "  ")
	requireWikiParent(t, r, proj.Identifier, child, "")

	// A parent named by a renamed page's old title: Redmine follows the
	// rename redirect, so the move lands. The API cannot resolve the redirect,
	// so the CLI must not report parent_not_applied; at most it warns.
	renamed := "Renamed_" + suffix
	r.run(t, "wiki", "update", other, "--project", proj.Identifier, "--title", renamed)
	stdout, stderrRedirect, err := r.runRaw("wiki", "update", child, "--project", proj.Identifier, "--parent", other)
	if err != nil {
		t.Fatalf("move via redirected parent title failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderrRedirect)
	}
	requireWikiParent(t, r, proj.Identifier, child, renamed)
	r.run(t, "wiki", "update", child, "--project", proj.Identifier, "--parent", "")

	// Unknown parent: Redmine answers a bare 422; the CLI must name the
	// parent instead of a message-less "Validation error".
	stderr := r.runExpectJSONError(t, "wiki", "update", child, "--project", proj.Identifier,
		"--parent", "Missing_"+suffix)
	assertErrorCode(t, stderr, "validation_failed")
	env := requireErrorEnvelopeMessage(t, stderr)
	if len(env.Error.Details) == 0 || !strings.Contains(env.Error.Details[0], "Missing_"+suffix) {
		t.Errorf("unknown parent: details = %q, want the rejected parent named", env.Error.Details)
	}
	stderr = r.runExpectJSONError(t, "wiki", "create", "New_"+suffix, "--project", proj.Identifier,
		"--text", "x", "--parent", "Missing_"+suffix)
	assertErrorCode(t, stderr, "validation_failed")

	// Circular: guide under child, then child under guide.
	r.run(t, "wiki", "update", guide, "--project", proj.Identifier, "--parent", child)
	stderr = r.runExpectJSONError(t, "wiki", "update", child, "--project", proj.Identifier, "--parent", guide)
	assertErrorCode(t, stderr, "validation_failed")
	requireWikiParent(t, r, proj.Identifier, child, "")
}

// TestWiki_ParentNotApplied_NonAdmin uses a Developer (edit_wiki_pages but no
// rename_wiki_pages). Redmine drops parent_title on existing pages for such a
// user and still answers 204, so the CLI must fail with parent_not_applied
// while new pages still get their parent.
func TestWiki_ParentNotApplied_NonAdmin(t *testing.T) {
	requireE2E(t)
	admin := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, admin)
	member := createTestUser(t, admin)
	addProjectMember(t, admin, proj.Identifier, "--user-id", member.ID, roleIDByName(t, admin, "Developer"))
	r := newCLIRunnerBasicAuth(t, e2eBaseURL(), member.Login, member.Password)

	suffix := strconv.FormatInt(time.Now().UnixNano(), 36)
	guide := "Guide_" + suffix
	page := "Page_" + suffix
	admin.run(t, "wiki", "create", guide, "--project", proj.Identifier, "--text", "Guide.")
	admin.run(t, "wiki", "create", page, "--project", proj.Identifier, "--text", "Old body.")

	// A new page is a new record, which Redmine lets any editor parent.
	var created wikiParent
	r.runJSON(t, &created, "wiki", "create", "New_"+suffix, "--project", proj.Identifier,
		"--text", "New.", "--parent", guide)
	if created.parentTitle() != guide {
		t.Fatalf("non-admin create: parent = %q, want %q", created.parentTitle(), guide)
	}

	stderr := r.runExpectJSONError(t, "wiki", "update", page, "--project", proj.Identifier,
		"--text", "New body.", "--parent", guide)
	assertErrorCode(t, stderr, "parent_not_applied")
	env := requireErrorEnvelopeMessage(t, stderr)
	for _, want := range []string{guide, "rename_wiki_pages"} {
		if !strings.Contains(env.Error.Message, want) {
			t.Errorf("parent_not_applied message missing %q: %q", want, env.Error.Message)
		}
	}
	// The text change in the same request was applied, as the details say.
	after := requireWikiParent(t, admin, proj.Identifier, page, "")
	if after.Text != "New body." {
		t.Errorf("text after dropped parent = %q, want the new body", after.Text)
	}

	// create over an existing page is an update for Redmine and is checked too.
	stderr = r.runExpectJSONError(t, "wiki", "create", page, "--project", proj.Identifier,
		"--text", "Overwritten.", "--parent", guide)
	assertErrorCode(t, stderr, "parent_not_applied")

	// The rename is dropped under the same rule, so the page is still found
	// under its old title and the dropped rename is reported too.
	stderr = r.runExpectJSONError(t, "wiki", "update", page, "--project", proj.Identifier,
		"--title", "Moved_"+suffix, "--parent", guide)
	assertErrorCode(t, stderr, "parent_not_applied")
	env = requireErrorEnvelopeMessage(t, stderr)
	if !strings.Contains(strings.Join(env.Error.Details, " "), "Moved_"+suffix) {
		t.Errorf("parent_not_applied details do not mention the dropped rename: %q", env.Error.Details)
	}
	requireWikiParent(t, admin, proj.Identifier, page, "")
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
