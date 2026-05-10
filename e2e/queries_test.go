//go:build e2e

package e2e

import (
	"strconv"
	"testing"
)

// TestQueries_ListAndFilter exercises the queries surface end-to-end. Redmine
// does not expose a REST endpoint to create saved queries, so the test only
// asserts that:
//
//   - `queries list` returns a well-formed (possibly empty) JSON array.
//   - `issues list --query-id <id>` accepts the flag and runs successfully.
//     When the server has no saved queries, the test still verifies that the
//     CLI threads --query-id through without parsing errors by passing a
//     positive integer; Redmine returns the unfiltered issue list when the
//     query_id is not a real saved query, which is exactly the wire-format
//     check we want.
func TestQueries_ListAndFilter(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	var queries []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	r.runJSON(t, &queries, "queries", "list")

	queryID := 1
	if len(queries) > 0 {
		queryID = queries[0].ID
	}

	var issues []struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &issues, "issues", "list", "--query-id", strconv.Itoa(queryID), "--limit", "5")
}
