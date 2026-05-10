//go:build e2e

package e2e

import (
	"strconv"
	"testing"
)

// TestQueries_ListAndFilter exercises the queries surface end-to-end. Redmine
// does not expose a REST endpoint to create saved queries, so the test only
// asserts that `queries list` returns a well-formed (possibly empty) JSON
// array. When the bootstrap data set happens to include a saved query
// (depends on the Redmine version) the test additionally runs
// `issues list --query-id <id>` to verify the filter is wired end-to-end.
// Passing a non-existent query_id is skipped because Redmine returns a 404
// in that case, which would mask real regressions.
func TestQueries_ListAndFilter(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var queries []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r.runJSON(t, &queries, "queries", "list")

	if len(queries) == 0 {
		t.Skip("no saved queries on this Redmine instance; skipping --query-id wire check")
	}

	var issues []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &issues, "issues", "list", "--query-id", strconv.Itoa(queries[0].ID), "--limit", "5")
}
