//go:build e2e

package e2e

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// TestJQ_FiltersOutput exercises --jq across resource bodies, lists, action
// envelopes and the raw api passthrough against a live server.
func TestJQ_FiltersOutput(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssueWithSubject(t, r, proj.Identifier, "jq subject "+uniqueShortSuffix(t))
	id := strconv.Itoa(issue.ID)

	tests := []struct {
		name string
		args []string
		want string
	}{
		{"resource string is raw", []string{"issues", "get", id, "--jq", ".subject"}, issue.Subject + "\n"},
		{"resource number", []string{"issues", "get", id, "-q", ".id"}, id + "\n"},
		{"list", []string{"issues", "list", "--project", proj.Identifier, "--jq", ".[].id"}, id + "\n"},
		{"empty list", []string{"issues", "list", "--project", proj.Identifier, "--status", "closed", "--jq", "."}, "[]\n"},
		{"object is compact", []string{"issues", "get", id, "--jq", "{id}"}, fmt.Sprintf(`{"id":%d}`, issue.ID) + "\n"},
		{"api body", []string{"api", "/issues/" + id + ".json", "--jq", ".issue.subject"}, issue.Subject + "\n"},
		{"action envelope", []string{"issues", "comment", id, "--message", "via jq", "--jq", ".action"}, "commented\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(r.run(t, tt.args...))
			if got != tt.want {
				t.Fatalf("stdout = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestJQ_FailedWriteKeepsStdoutEmpty reproduces the agent pattern
// `issues create ... --jq '{id}'`: a rejected create must leave stdout empty
// and put the unfiltered error envelope on stderr.
func TestJQ_FailedWriteKeepsStdoutEmpty(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	stderr := r.runExpectJSONError(t, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", firstTrackerName(t, r),
		"--subject", "",
		"--jq", "{id}")
	env := requireErrorEnvelopeMessage(t, stderr)
	if env.Error.Code != "validation_failed" {
		t.Fatalf("error code = %q, want validation_failed\nstderr:\n%s", env.Error.Code, stderr)
	}
}

// TestJQ_RejectsTableOutput verifies --jq refuses an explicit non-JSON format.
func TestJQ_RejectsTableOutput(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	stdout, stderr := r.runExpectError(t, "users", "me", "--output", "table", "--jq", ".id")
	if len(stdout) != 0 {
		t.Fatalf("expected empty stdout, got:\n%s", stdout)
	}
	if !strings.Contains(string(stderr), "--jq requires JSON output") {
		t.Fatalf("unexpected stderr:\n%s", stderr)
	}
}
