//go:build e2e

package e2e

import (
	"strconv"
	"strings"
	"testing"
)

// seededQueryName matches the saved query inserted by e2e/bootstrap-redmine.sh.
// Tests rely on this fixture because Redmine has no REST endpoint to create
// saved queries, so the bootstrap is the only seeding hook the harness has.
const seededQueryName = "E2E All Open Issues"

// TestQueries_ListIncludesSeededFixture asserts that the saved query the
// bootstrap script seeds is visible to `redmine queries list`. Together with
// the issues-list test below this gives end-to-end coverage of both the
// `queries` group and the `--query`/`--query-id` filter wiring.
func TestQueries_ListIncludesSeededFixture(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	id := requireSeededQueryID(t, r)
	if id <= 0 {
		t.Fatalf("seeded query ID = %d, want positive", id)
	}
}

// TestIssues_ListByQueryID exercises the happy path for the `--query-id` flag
// against a real saved query. Redmine returns the query's filtered result
// set; we only assert that the call succeeded and produced a JSON array
// because the fixture has no committed issues yet, so an empty list is
// valid.
func TestIssues_ListByQueryID(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	id := requireSeededQueryID(t, r)

	var issues []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &issues, "issues", "list", "--query-id", strconv.Itoa(id), "--limit", "5")
}

// TestIssues_ListByQueryName proves that `--query` resolves a name to an ID
// against the live Redmine instance and that the resolved ID then drives a
// successful issues list call.
func TestIssues_ListByQueryName(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	// Verify the seed exists before asserting on the name path so a missing
	// fixture surfaces as a clear setup error rather than a name-resolution
	// failure deep inside the resolver.
	_ = requireSeededQueryID(t, r)

	var issues []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &issues, "issues", "list", "--query", seededQueryName, "--limit", "5")
}

// TestIssues_ListByUnknownQueryID verifies the negative path: passing a
// query_id that no saved query owns must surface as a not_found error
// envelope. This proves the parameter actually reaches Redmine (rather than
// being silently dropped by the CLI) and that the error is mapped through
// the standard envelope plumbing.
func TestIssues_ListByUnknownQueryID(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	stdout, _ := r.runExpectError(t, "issues", "list", "--query-id", "2147483600")
	assertErrorCode(t, stdout, "not_found")
}

// TestQueries_GetRoundTrip exercises `queries get` against the seeded fixture
// and verifies the detail rendering reports the query as public + global.
func TestQueries_GetRoundTrip(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	id := requireSeededQueryID(t, r)

	stdout := r.run(t, "queries", "get", strconv.Itoa(id), "--output", "table")
	for _, want := range []string{seededQueryName, "public", "global"} {
		if !strings.Contains(string(stdout), want) {
			t.Fatalf("queries get output missing %q\nstdout:\n%s", want, stdout)
		}
	}
}

// requireSeededQueryID returns the ID of the saved query named seededQueryName
// or fails the test. Tests use this rather than searching inline so a missing
// bootstrap fixture surfaces with a single, descriptive failure.
func requireSeededQueryID(t *testing.T, r *cliRunner) int {
	t.Helper()
	var queries []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r.runJSON(t, &queries, "queries", "list")

	for _, q := range queries {
		if q.Name == seededQueryName {
			return q.ID
		}
	}
	t.Fatalf("seeded query %q not found in queries list (did the bootstrap run?); got: %+v", seededQueryName, queries)
	return 0
}
