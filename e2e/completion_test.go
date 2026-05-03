//go:build e2e

package e2e

import (
	"bytes"
	"strings"
	"testing"
)

// TestCompletion_Generates exercises every shell that the completion command
// supports. The cobra-generated output never contacts Redmine but we keep the
// tests under the e2e build tag so TestMain's binary build is amortised.
//
// For each shell we assert one structural marker (proves cobra ran for that
// shell) plus, where the output is statically templated, a known subcommand
// name. zsh's completion is dynamic and does not embed subcommands, so we
// only assert the static markers there.
func TestCompletion_Generates(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	cases := []struct {
		shell   string
		markers []string
	}{
		{"bash", []string{"__start_redmine", "_redmine_issues", "_redmine_projects"}},
		{"zsh", []string{"#compdef redmine", "_redmine"}},
		{"fish", []string{"complete -c redmine", "issues"}},
		{"powershell", []string{"Register-ArgumentCompleter", "redmine"}},
	}
	for _, tc := range cases {
		t.Run(tc.shell, func(t *testing.T) {
			stdout := r.run(t, "completion", tc.shell)
			if len(bytes.TrimSpace(stdout)) == 0 {
				t.Fatalf("completion %s: stdout is empty", tc.shell)
			}
			got := string(stdout)
			for _, marker := range tc.markers {
				if !strings.Contains(got, marker) {
					t.Errorf("completion %s: stdout missing %q\nstdout:\n%s",
						tc.shell, marker, got)
				}
			}
		})
	}
}

// TestCompletion_Rejects covers the cobra arg-validation paths: an unknown
// shell name and a missing arg both exit non-zero before script generation.
func TestCompletion_Rejects(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	t.Run("invalid shell", func(t *testing.T) {
		r.runExpectError(t, "completion", "not-a-shell")
	})
	t.Run("no args", func(t *testing.T) {
		r.runExpectError(t, "completion")
	})
}
