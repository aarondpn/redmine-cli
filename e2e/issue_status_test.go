//go:build e2e

package e2e

import (
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

// TestIssues_StatusNotAppliedIsReported closes a parent issue that still has
// an open subtask. Redmine refuses that transition silently (it answers the
// PUT with 204 and keeps the old status), so the CLI must read the issue
// back, report ok:false, explain on stderr and exit non-zero. This is the
// same code path as a workflow-forbidden transition, but reproducible as the
// admin user, who is otherwise allowed every workflow transition.
func TestIssues_StatusNotAppliedIsReported(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	parent := createTestIssueWithSubject(t, r, proj.Identifier, "E2E parent with open subtask")

	var child struct {
		ID int `json:"id"`
	}
	r.runJSON(t, &child, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", firstTrackerName(t, r),
		"--subject", "E2E open subtask",
		"--parent", issueIDArg(parent.ID))
	if child.ID == 0 {
		t.Fatal("subtask create returned no ID")
	}

	before := getIssueStatus(t, r, parent.ID)
	note := "Close attempt by e2e " + issueIDArg(parent.ID)
	stdout, stderr, err := r.runRaw("issues", "close", issueIDArg(parent.ID), "--note", note)
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != 1 {
		t.Fatalf("expected exit code 1, got err=%v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	var env actionEnvelope
	if err := json.Unmarshal(stdout, &env); err != nil {
		t.Fatalf("stdout is not an action envelope: %v\n%s", err, stdout)
	}
	if env.Ok || env.Action != "closed" || env.Resource != "issue" || envelopeIntID(env.ID) != parent.ID {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if !strings.Contains(env.Message, "status was not changed") {
		t.Errorf("envelope message = %q", env.Message)
	}

	errText := string(stderr)
	if !strings.Contains(errText, "status was not changed") || !strings.Contains(errText, "may have been applied") {
		t.Errorf("stderr missing the explanation:\n%s", errText)
	}
	switch {
	case redmineAtLeast(5, 0):
		if !strings.Contains(errText, "Allowed statuses:") {
			t.Errorf("Redmine %s supports include=allowed_statuses; stderr should list them:\n%s", e2eVersion(), errText)
		}
	case e2eVersion() != "":
		if !strings.Contains(errText, "Redmine 5.0+") {
			t.Errorf("older Redmine should get the graceful fallback hint:\n%s", errText)
		}
	}

	if after := getIssueStatus(t, r, parent.ID); after != before {
		t.Fatalf("parent status changed from %q to %q; expected Redmine to refuse the close", before, after)
	}
	// The note travels with the dropped status change and is still applied,
	// which is exactly why the message warns against resending blindly.
	if !issueHasNote(t, r, parent.ID, note) {
		t.Errorf("expected the close note to be recorded despite the ignored status change")
	}

	// Closing the subtask first makes the same transition legal again.
	var closedChild actionEnvelope
	r.runJSON(t, &closedChild, "issues", "close", issueIDArg(child.ID))
	if !closedChild.Ok {
		t.Fatalf("closing the subtask should succeed: %+v", closedChild)
	}
	var closedParent actionEnvelope
	r.runJSON(t, &closedParent, "issues", "close", issueIDArg(parent.ID))
	if !closedParent.Ok {
		t.Fatalf("closing the parent after its subtask should succeed: %+v", closedParent)
	}
}

// issueHasNote reports whether one of the issue's journals carries exactly
// the given notes.
func issueHasNote(t *testing.T, r *cliRunner, id int, notes string) bool {
	t.Helper()
	var got struct {
		Journals []struct {
			Notes string `json:"notes"`
		} `json:"journals"`
	}
	r.runJSON(t, &got, "issues", "get", issueIDArg(id), "--journals")
	for _, j := range got.Journals {
		if normalizeNewlines(j.Notes) == notes {
			return true
		}
	}
	return false
}

// normalizeNewlines undoes Redmine's server-side conversion of stored text
// to CRLF line endings, so tests can compare against what the CLI sent.
func normalizeNewlines(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}
