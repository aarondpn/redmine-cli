//go:build e2e

package e2e

import (
	"bytes"
	"strings"
	"testing"
)

// The completion command is built from cobra metadata and never contacts the
// Redmine server, but we still gate behind requireE2E for consistency with the
// rest of the suite (and so the binary build in TestMain is amortized across
// all tests).

// TestCompletion_BashGenerates verifies that `redmine completion bash`
// produces a non-empty bash completion script that references the binary,
// the bash shape, and at least one known subcommand.
func TestCompletion_BashGenerates(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	stdout := r.run(t, "completion", "bash")
	assertCompletionScript(t, "bash", stdout, "complete -F", "redmine", "issues", "projects")
}

// TestCompletion_ZshGenerates verifies that `redmine completion zsh` produces
// a non-empty zsh completion script with the expected zsh shape.
func TestCompletion_ZshGenerates(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	stdout := r.run(t, "completion", "zsh")
	assertCompletionScript(t, "zsh", stdout, "#compdef redmine", "_redmine", "issues")
}

// TestCompletion_FishGenerates verifies that `redmine completion fish`
// produces a non-empty fish completion script with the expected fish shape.
func TestCompletion_FishGenerates(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	stdout := r.run(t, "completion", "fish")
	assertCompletionScript(t, "fish", stdout, "complete -c redmine", "issues")
}

// TestCompletion_PowerShellGenerates verifies that `redmine completion
// powershell` produces a non-empty PowerShell completion script.
func TestCompletion_PowerShellGenerates(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	stdout := r.run(t, "completion", "powershell")
	assertCompletionScript(t, "powershell", stdout, "Register-ArgumentCompleter", "redmine")
}

// TestCompletion_InvalidShell asserts that cobra rejects an unknown shell with
// a non-zero exit. The completion command uses cobra.OnlyValidArgs so any
// value outside {bash,zsh,fish,powershell} must fail before script generation.
func TestCompletion_InvalidShell(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	r.runExpectError(t, "completion", "not-a-shell")
}

// TestCompletion_NoArgs asserts that omitting the shell argument fails the
// cobra.ExactArgs(1) gate.
func TestCompletion_NoArgs(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	r.runExpectError(t, "completion")
}

// assertCompletionScript fails the test if stdout is empty or any required
// marker is missing.
func assertCompletionScript(t *testing.T, shell string, stdout []byte, markers ...string) {
	t.Helper()
	if len(bytes.TrimSpace(stdout)) == 0 {
		t.Fatalf("completion %s: stdout is empty", shell)
	}
	got := string(stdout)
	for _, marker := range markers {
		if !strings.Contains(got, marker) {
			t.Fatalf("completion %s: stdout missing %q\nstdout:\n%s",
				shell, marker, got)
		}
	}
}
