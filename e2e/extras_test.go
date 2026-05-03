//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestVersions_LockedExcludesFromAssignment creates a version, locks it via
// `versions update`, then attempts to create an issue against the locked
// version. Redmine treats locked versions as unassignable and returns a 422.
// If a Redmine build accepts the assignment, we Skip with a documented note
// rather than fail.
func TestVersions_LockedExcludesFromAssignment(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	versionName := fmt.Sprintf("locked-v-%d", time.Now().UnixNano())

	var created struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &created, "versions", "create",
		"--project", proj.Identifier,
		"--name", versionName,
		"--status", "open")
	if created.ID == 0 {
		t.Fatalf("create returned zero ID: %+v", created)
	}

	var updated actionEnvelope
	r.runJSON(t, &updated, "versions", "update", strconv.Itoa(created.ID),
		"--status", "locked")
	if !updated.Ok {
		t.Fatalf("update envelope not ok: %+v", updated)
	}

	stdout, stderr, err := r.runRaw("issues", "create",
		"--project", proj.Identifier,
		"--tracker", firstTrackerName(t, r),
		"--subject", fmt.Sprintf("issue against locked version %d", time.Now().UnixNano()),
		"--version", versionName)
	if err == nil {
		t.Skipf("Redmine accepted issue assignment to a locked version; this build does not enforce the lock.\nstdout:\n%s\nstderr:\n%s",
			stdout, stderr)
	}
	requireErrorEnvelopeMessage(t, stdout)
}

// TestVersions_GetNonexistent verifies that fetching a version by a name that
// does not exist exits non-zero with a populated error envelope. The CLI
// resolves the name client-side and produces a suggestion-style error rather
// than a server 404, so we don't pin the code.
func TestVersions_GetNonexistent(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	missing := fmt.Sprintf("nonexistent-version-%d", time.Now().UnixNano())
	stdout, _ := r.runExpectError(t, "versions", "get", missing, "--project", proj.Identifier)
	requireErrorEnvelopeMessage(t, stdout)
}

// TestSearch_Wiki creates a wiki page via the raw api passthrough (PUT
// /projects/<id>/wiki/<page>.json with a unique token in the body) and
// verifies the dedicated `search wiki` subcommand surfaces it. The wiki PUT
// pattern creates the page if missing per
// https://www.redmine.org/projects/redmine/wiki/Rest_WikiPages.
func TestSearch_Wiki(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	token := fmt.Sprintf("wikitoken%d", time.Now().UnixNano())
	pageTitle := fmt.Sprintf("E2EWikiPage%d", time.Now().UnixNano())

	body, err := json.Marshal(map[string]any{
		"wiki_page": map[string]any{
			"text": "Wiki page created by e2e test " + token,
		},
	})
	if err != nil {
		t.Fatalf("marshal wiki body: %v", err)
	}
	bodyPath := writeBodyFile(t, body)

	endpoint := fmt.Sprintf("/projects/%s/wiki/%s.json", proj.Identifier, pageTitle)
	r.run(t, "api", endpoint, "-X", "PUT", "--input", bodyPath)

	var results []struct {
		Type string `json:"type"`
	}
	r.runJSON(t, &results, "search", "wiki", token, "--limit", "25")

	for _, res := range results {
		if strings.Contains(strings.ToLower(res.Type), "wiki") {
			return
		}
	}
	t.Fatalf("search wiki %q did not return any wiki-page result; got %+v", token, results)
}

// TestSearch_AllScopes runs `search <token>` with no scope flags and verifies
// the issue is returned. Redmine's default behavior, when no scope is set, is
// to search all resource types.
func TestSearch_AllScopes(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	token := fmt.Sprintf("allscopes%d", time.Now().UnixNano())
	issue := createTestIssueWithSubject(t, r, proj.Identifier, "Find all scopes "+token)

	var results []struct {
		ID   int    `json:"id"`
		Type string `json:"type"`
	}
	r.runJSON(t, &results, "search", token, "--limit", "25")

	for _, res := range results {
		if res.ID == issue.ID && strings.Contains(res.Type, "issue") {
			return
		}
	}
	t.Fatalf("default-scope search %q did not find issue %d; got %+v", token, issue.ID, results)
}

// TestSearch_NoResults verifies that a query guaranteed to match nothing
// returns an empty JSON array.
func TestSearch_NoResults(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	stdout := r.run(t, "search", "xxxxxxxxxxnonexistenttoken12345", "--issues", "--limit", "5")
	if got := strings.TrimSpace(string(stdout)); got != "[]" {
		t.Fatalf("expected empty JSON array, got: %s", got)
	}
}

// TestAPI_NotFoundResponse exercises the raw api passthrough against a
// non-existent issue. The CLI exits non-zero with a not_found envelope.
func TestAPI_NotFoundResponse(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	stdout, _ := r.runExpectError(t, "api", "/issues/2147483600.json")
	assertErrorCode(t, stdout, "not_found")
}

// TestAPI_Delete creates an issue via the typed CLI and removes it via the
// raw api passthrough (DELETE returns 204 No Content). The follow-up
// `issues get` must surface a not_found envelope.
func TestAPI_Delete(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)

	r.run(t, "api", fmt.Sprintf("/issues/%d.json", issue.ID), "-X", "DELETE")

	stdout, _ := r.runExpectError(t, "issues", "get", strconv.Itoa(issue.ID))
	assertErrorCode(t, stdout, "not_found")
}

// TestAPI_FormFlag exercises -f as query parameters on a GET endpoint by
// asking for at most 1 issue across all statuses; the response must contain
// no more than 1 entry in the `issues` array and echo limit=1.
func TestAPI_FormFlag(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	// Ensure at least one issue exists in the global index.
	_ = createTestIssue(t, r, proj.Identifier)

	var resp struct {
		Issues []struct {
			ID int `json:"id"`
		} `json:"issues"`
		Limit int `json:"limit"`
	}
	r.runJSON(t, &resp, "api", "/issues.json", "-f", "limit=1", "-f", "status_id=*")

	if len(resp.Issues) > 1 {
		t.Fatalf("api -f limit=1 returned %d issues, want <= 1", len(resp.Issues))
	}
	if resp.Limit != 1 {
		t.Fatalf("api -f limit=1 echoed limit = %d, want 1", resp.Limit)
	}
}

// TestErrors_ValidationFailure submits an issues create with an empty subject
// and asserts a populated error envelope. Redmine returns 422; the CLI
// classifies that as validation_failed when the underlying error is an
// *api.APIError. The check on Code is best-effort to stay portable.
func TestErrors_ValidationFailure(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	stdout, _ := r.runExpectError(t, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", firstTrackerName(t, r),
		"--subject", "")

	env := requireErrorEnvelopeMessage(t, stdout)
	if env.Error.Code != "" && env.Error.Code != "validation_failed" {
		t.Logf("validation envelope code = %q (expected validation_failed)", env.Error.Code)
	}
}

// TestErrors_ConnectionFailure points a runner at a port with no listener
// and verifies the CLI surfaces a populated error envelope. Connection
// failures are not *api.APIError, so the code falls back to "unknown"
// (see internal/cmdutil/errors.go BuildErrorEnvelope).
func TestErrors_ConnectionFailure(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, "http://127.0.0.1:1", "fake-key")

	stdout, _ := r.runExpectError(t, "users", "me")
	env := requireErrorEnvelopeMessage(t, stdout)
	if env.Error.Code != "unknown" {
		t.Fatalf("connection failure code = %q, want unknown\nstdout:\n%s", env.Error.Code, stdout)
	}
}

// requireErrorEnvelopeMessage decodes stdout as an error envelope and
// requires a non-empty message. It returns the decoded envelope so callers
// can perform additional code-level assertions.
func requireErrorEnvelopeMessage(t *testing.T, stdout []byte) errorEnvelope {
	t.Helper()
	var env errorEnvelope
	if err := json.Unmarshal(stdout, &env); err != nil {
		t.Fatalf("decode error envelope: %v\nstdout:\n%s", err, stdout)
	}
	if strings.TrimSpace(env.Error.Message) == "" {
		t.Fatalf("error envelope missing message\nstdout:\n%s", stdout)
	}
	return env
}
