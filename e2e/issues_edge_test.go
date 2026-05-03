//go:build e2e

package e2e

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// TestIssuesList_PaginationOffset creates three issues in a fresh project and
// verifies that --limit / --offset paging is honoured by the server.
func TestIssuesList_PaginationOffset(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	want := []int{
		createTestIssueWithSubject(t, r, proj.Identifier, "Pagination issue 1").ID,
		createTestIssueWithSubject(t, r, proj.Identifier, "Pagination issue 2").ID,
		createTestIssueWithSubject(t, r, proj.Identifier, "Pagination issue 3").ID,
	}

	page1 := listIssueIDs(t, r, "--project", proj.Identifier, "--status", "*",
		"--limit", "2", "--offset", "0")
	if len(page1) != 2 {
		t.Fatalf("first page len = %d, want 2; ids=%v", len(page1), page1)
	}
	for _, id := range page1 {
		if !containsInt(want, id) {
			t.Fatalf("first page id %d is not one of the created issues %v", id, want)
		}
	}

	page2 := listIssueIDs(t, r, "--project", proj.Identifier, "--status", "*",
		"--limit", "2", "--offset", "2")
	if len(page2) != 1 {
		t.Fatalf("second page len = %d, want 1; ids=%v", len(page2), page2)
	}
	if !containsInt(want, page2[0]) {
		t.Fatalf("second page id %d is not one of the created issues %v", page2[0], want)
	}
	if containsInt(page1, page2[0]) {
		t.Fatalf("second page id %d overlaps first page %v", page2[0], page1)
	}

	farPage := listIssueIDs(t, r, "--project", proj.Identifier, "--status", "*",
		"--limit", "10", "--offset", "100")
	if len(farPage) != 0 {
		t.Fatalf("offset=100 should yield empty page; got %v", farPage)
	}
}

// TestIssuesList_SortDescending verifies that --sort id:desc is forwarded to
// the server and the returned issues come back in descending ID order.
func TestIssuesList_SortDescending(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	created := []int{
		createTestIssueWithSubject(t, r, proj.Identifier, "Sort issue 1").ID,
		createTestIssueWithSubject(t, r, proj.Identifier, "Sort issue 2").ID,
		createTestIssueWithSubject(t, r, proj.Identifier, "Sort issue 3").ID,
	}

	got := listIssueIDs(t, r, "--project", proj.Identifier, "--status", "*",
		"--sort", "id:desc", "--limit", "10")
	if len(got) < len(created) {
		t.Fatalf("expected at least %d ids, got %d (%v)", len(created), len(got), got)
	}

	for i := 1; i < len(got); i++ {
		if got[i-1] < got[i] {
			t.Fatalf("results not sorted desc by id at index %d: %v", i, got)
		}
	}

	for _, id := range created {
		if !containsInt(got, id) {
			t.Fatalf("sort result missing created issue %d; got %v", id, got)
		}
	}
}

// TestIssuesList_IncludeRelations creates a relation between two issues and
// verifies that `--include relations` causes the JSON output to expose the
// `relations` array on at least one of the related issues.
func TestIssuesList_IncludeRelations(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	a := createTestIssueWithSubject(t, r, proj.Identifier, "Relation source").ID
	b := createTestIssueWithSubject(t, r, proj.Identifier, "Relation target").ID

	body, _ := json.Marshal(map[string]any{
		"relation": map[string]any{
			"issue_to_id":   b,
			"relation_type": "relates",
		},
	})
	bodyPath := writeBodyFile(t, body)
	r.run(t, "api", fmt.Sprintf("/issues/%d/relations.json", a),
		"-X", "POST", "--input", bodyPath)

	// Decode permissively: the typed Issue model omits `relations`, so use
	// map[string]any and look for a non-empty relations array on one of the
	// two created issues.
	var raw []map[string]any
	r.runJSON(t, &raw, "issues", "list",
		"--project", proj.Identifier, "--status", "*",
		"--include", "relations", "--limit", "100")

	if !hasRelationsForIssue(raw, a) && !hasRelationsForIssue(raw, b) {
		t.Fatalf("expected `relations` field on issue %d or %d in list output; got %+v", a, b, raw)
	}
}

func hasRelationsForIssue(raw []map[string]any, id int) bool {
	for _, item := range raw {
		idVal, ok := item["id"].(float64)
		if !ok || int(idVal) != id {
			continue
		}
		rels, ok := item["relations"].([]any)
		if ok && len(rels) > 0 {
			return true
		}
	}
	return false
}

// TestIssuesList_VersionFilter creates a version, tags one issue with it, and
// verifies that --version filters the list down to exactly that issue.
func TestIssuesList_VersionFilter(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	const versionName = "edge-version-1"
	var version struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r.runJSON(t, &version, "versions", "create",
		"--project", proj.Identifier,
		"--name", versionName,
		"--status", "open")
	if version.ID == 0 {
		t.Fatalf("created version has zero ID: %+v", version)
	}

	tracker := firstTrackerName(t, r)
	var tagged struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &tagged, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", tracker,
		"--subject", "Issue tagged with version",
		"--version", versionName)
	if tagged.ID == 0 {
		t.Fatal("issues create returned no ID for version-tagged issue")
	}

	untagged := createTestIssueWithSubject(t, r, proj.Identifier, "Issue without version").ID

	ids := listIssueIDs(t, r, "--project", proj.Identifier, "--status", "*",
		"--version", versionName, "--limit", "100")
	if !containsInt(ids, tagged.ID) {
		t.Fatalf("version filter should include tagged issue %d; got %v", tagged.ID, ids)
	}
	if containsInt(ids, untagged) {
		t.Fatalf("version filter should exclude untagged issue %d; got %v", untagged, ids)
	}
}

// TestIssuesCreate_ResolverFailure verifies that an unknown assignee surfaces
// as a clear "resolving assignee" / "not found" error rather than being
// swallowed by the server.
func TestIssuesCreate_ResolverFailure(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	tracker := firstTrackerName(t, r)

	stdout, stderr := r.runExpectError(t, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", tracker,
		"--subject", "Resolver failure",
		"--assignee", "this-user-does-not-exist-e2e")

	combined := strings.ToLower(string(stdout) + "\n" + string(stderr))
	if !strings.Contains(combined, "resolving assignee") &&
		!strings.Contains(combined, "not found") &&
		!strings.Contains(combined, "no match") {
		t.Fatalf("expected resolver error mentioning assignee; stdout:\n%s\nstderr:\n%s", stdout, stderr)
	}
}

// TestIssuesCreate_EmptySubjectValidationError verifies that the server-side
// validation failure for an empty subject is surfaced as a non-zero exit and
// a populated error envelope on stdout.
func TestIssuesCreate_EmptySubjectValidationError(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	tracker := firstTrackerName(t, r)

	stdout, _ := r.runExpectError(t, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", tracker,
		"--subject", "")

	var env errorEnvelope
	if err := json.Unmarshal(stdout, &env); err != nil {
		t.Fatalf("decode error envelope: %v\nstdout:\n%s", err, stdout)
	}
	if strings.TrimSpace(env.Error.Message) == "" {
		t.Fatalf("error envelope missing message; stdout:\n%s", stdout)
	}
	// Different Redmine versions classify "blank subject" differently
	// (validation_failed, server_error, or no code at all). Accept any of
	// those rather than coupling to a single backend version.
	switch env.Error.Code {
	case "", "validation_failed", "server_error", "unknown":
	default:
		t.Fatalf("unexpected error code %q for empty subject; want validation_failed/server_error/unknown\nstdout:\n%s",
			env.Error.Code, stdout)
	}
}

// TestIssuesUpdate_ClearAssignee documents the limitation that the CLI does
// not currently support clearing an assignee via `--assignee ""`. The empty
// string is fed to resolver.ResolveAssignee (see
// internal/cmd/issue/update.go and internal/resolver/resolver.go), which
// treats it as a name lookup and fails. If the CLI ever grows a sentinel for
// "unassign", replace this Skip with a real round-trip assertion.
func TestIssuesUpdate_ClearAssignee(t *testing.T) {
	requireE2E(t)
	t.Skip("CLI does not currently support clearing assignee via --assignee \"\"; " +
		"see internal/cmd/issue/update.go and internal/resolver/resolver.go. " +
		"Update this test if a clear/unset sentinel is added.")
}
