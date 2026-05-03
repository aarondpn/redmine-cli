//go:build e2e

package e2e

import "testing"

// TestIssues_Delete creates an issue, deletes it via `issues delete --force`,
// then confirms a follow-up `issues get` returns a not_found error envelope.
func TestIssues_Delete(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)

	var deleted actionEnvelope
	r.runJSON(t, &deleted, "issues", "delete", issueIDArg(issue.ID), "--force")
	if !deleted.Ok || deleted.Action != "deleted" || deleted.Resource != "issue" || envelopeIntID(deleted.ID) != issue.ID {
		t.Fatalf("unexpected delete envelope: %+v", deleted)
	}

	stdout, _ := r.runExpectError(t, "issues", "get", issueIDArg(issue.ID))
	assertErrorCode(t, stdout, "not_found")
}
