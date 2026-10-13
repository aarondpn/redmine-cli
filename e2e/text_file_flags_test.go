//go:build e2e

package e2e

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestIssues_TextFileFlags round-trips long, shell-hostile Textile through
// --description-file, --note-file (stdin) and --message-file.
func TestIssues_TextFileFlags(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	description := "h2. Steps\n\n# run @$(whoami)@\n# quote \"this\" and 'that'\n\nUmlaute äöü ✓\n\n<pre>\nif [ -n \"$X\" ]; then echo `date`; fi\n</pre>"
	descPath := filepath.Join(t.TempDir(), "description.textile")
	if err := os.WriteFile(descPath, []byte(description), 0o600); err != nil {
		t.Fatal(err)
	}

	var created struct {
		ID          int    `json:"id"`
		Description string `json:"description"`
	}
	r.runJSON(t, &created, "issues", "create",
		"--project", proj.Identifier,
		"--tracker", firstTrackerName(t, r),
		"--subject", "E2E file flags",
		"--description-file", descPath)
	if normalizeNewlines(created.Description) != description {
		t.Fatalf("description = %q, want %q", created.Description, description)
	}

	note := "Note from *stdin*\n\n* \"quoted\" $(not run)\n* ✓"
	stdout, stderr, err := runWithStdin(r, note, "issues", "update", issueIDArg(created.ID), "--note-file", "-")
	if err != nil {
		t.Fatalf("update --note-file -: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}

	comment := "Comment from file\n\nbc. code block with `backticks`"
	commentPath := filepath.Join(t.TempDir(), "comment.textile")
	if err := os.WriteFile(commentPath, []byte(comment), 0o600); err != nil {
		t.Fatal(err)
	}
	var commented actionEnvelope
	r.runJSON(t, &commented, "issues", "comment", issueIDArg(created.ID), "--message-file", commentPath)
	if !commented.Ok {
		t.Fatalf("unexpected comment envelope: %+v", commented)
	}

	for _, want := range []string{note, comment} {
		if !issueHasNote(t, r, created.ID, want) {
			t.Errorf("no journal with notes %q", want)
		}
	}

	exclusiveOut, exclusiveErr := r.runExpectError(t, "issues", "comment", issueIDArg(created.ID), "-m", "x", "--message-file", commentPath)
	if combined := string(exclusiveOut) + string(exclusiveErr); !strings.Contains(combined, "[message message-file] were all set") {
		t.Errorf("expected a mutual-exclusion error, got:\nstdout:\n%s\nstderr:\n%s", exclusiveOut, exclusiveErr)
	}
}

// runWithStdin runs the CLI with the given standard input.
func runWithStdin(r *cliRunner, stdin string, args ...string) (stdout, stderr []byte, err error) {
	cmdArgs := append([]string{"--config", r.configPath, "--output", "json"}, args...)
	cmd := exec.Command(builtCLIPath, cmdArgs...)
	cmd.Dir = r.repoRoot
	cmd.Env = append(os.Environ(), "REDMINE_NO_UPDATE_CHECK=1")
	cmd.Stdin = strings.NewReader(stdin)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	if runErr := cmd.Run(); runErr != nil {
		err = fmt.Errorf("run %q: %w", strings.Join(cmdArgs, " "), runErr)
	}
	return outBuf.Bytes(), errBuf.Bytes(), err
}
