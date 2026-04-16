//go:build e2e

package e2e

import (
	"strings"
	"testing"
)

// TestIssues_ListFilters exercises server-side filter round-trips: closed
// status, tracker filter, and the "me" assignee shortcut. Each filter should
// narrow the result set to the expected issue.
func TestIssues_ListFilters(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	open := createTestIssueWithSubject(t, r, proj.Identifier, "Open issue for filter test")
	closed := createTestIssueWithSubject(t, r, proj.Identifier, "Closed issue for filter test")

	var env actionEnvelope
	r.runJSON(t, &env, "issues", "close", issueIDArg(closed.ID))
	if !env.Ok {
		t.Fatalf("failed to close issue: %+v", env)
	}

	// Assign the open issue to the current user so --assignee me can match it.
	r.runJSON(t, &env, "issues", "assign", issueIDArg(open.ID), "me")

	t.Run("status=open excludes closed", func(t *testing.T) {
		ids := listIssueIDs(t, r, "--project", proj.Identifier, "--status", "open", "--limit", "100")
		if containsInt(ids, closed.ID) {
			t.Fatalf("status=open should exclude closed issue %d; got %v", closed.ID, ids)
		}
		if !containsInt(ids, open.ID) {
			t.Fatalf("status=open should include open issue %d; got %v", open.ID, ids)
		}
	})

	t.Run("status=closed excludes open", func(t *testing.T) {
		ids := listIssueIDs(t, r, "--project", proj.Identifier, "--status", "closed", "--limit", "100")
		if containsInt(ids, open.ID) {
			t.Fatalf("status=closed should exclude open issue %d; got %v", open.ID, ids)
		}
		if !containsInt(ids, closed.ID) {
			t.Fatalf("status=closed should include closed issue %d; got %v", closed.ID, ids)
		}
	})

	t.Run("status=* includes both", func(t *testing.T) {
		ids := listIssueIDs(t, r, "--project", proj.Identifier, "--status", "*", "--limit", "100")
		if !containsInt(ids, open.ID) || !containsInt(ids, closed.ID) {
			t.Fatalf("status=* should include both; got %v", ids)
		}
	})

	t.Run("assignee=me matches assigned issue", func(t *testing.T) {
		ids := listIssueIDs(t, r, "--project", proj.Identifier, "--status", "*",
			"--assignee", "me", "--limit", "100")
		if !containsInt(ids, open.ID) {
			t.Fatalf("assignee=me should include open issue %d; got %v", open.ID, ids)
		}
	})

	t.Run("tracker filter round-trips", func(t *testing.T) {
		tracker := firstTrackerName(t, r)
		ids := listIssueIDs(t, r, "--project", proj.Identifier, "--status", "*",
			"--tracker", tracker, "--limit", "100")
		// Both fixtures used the first tracker.
		if !containsInt(ids, open.ID) {
			t.Fatalf("tracker=%s should include open issue %d; got %v", tracker, open.ID, ids)
		}
	})

	t.Run("bogus tracker yields a resolver error", func(t *testing.T) {
		stdout, _ := r.runExpectError(t, "issues", "list", "--project", proj.Identifier,
			"--tracker", "this-tracker-does-not-exist")
		if !strings.Contains(string(stdout), "resolving tracker") &&
			!strings.Contains(string(stdout), "not found") {
			t.Fatalf("expected resolver error on unknown tracker; stdout:\n%s", stdout)
		}
	})
}

func listIssueIDs(t *testing.T, r *cliRunner, args ...string) []int {
	t.Helper()
	var listed []struct {
		ID int `json:"id"`
	}
	full := append([]string{"issues", "list"}, args...)
	r.runJSON(t, &listed, full...)
	out := make([]int, len(listed))
	for i, it := range listed {
		out[i] = it.ID
	}
	return out
}

func containsInt(xs []int, target int) bool {
	for _, x := range xs {
		if x == target {
			return true
		}
	}
	return false
}
