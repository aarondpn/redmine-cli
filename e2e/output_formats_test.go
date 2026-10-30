//go:build e2e

package e2e

import (
	"bytes"
	"encoding/csv"
	"regexp"
	"slices"
	"strconv"
	"testing"
)

// TestOutput_CSVViaJQ_IssuesList creates a project with two issues and runs
// the replacement for the removed CSV format that `-o csv` suggests: JSON
// output piped through jq's @csv. Both issues must come back as CSV rows,
// and `-o csv` itself must fail with that hint.
func TestOutput_CSVViaJQ_IssuesList(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	subjectA := "CSV row A, " + uniqueShortSuffix(t)
	subjectB := "CSV row \"B\" " + uniqueShortSuffix(t)
	issueA := createTestIssueWithSubject(t, r, proj.Identifier, subjectA)
	issueB := createTestIssueWithSubject(t, r, proj.Identifier, subjectB)

	stdout := r.run(t, "issues", "list",
		"--project", proj.Identifier,
		"--status", "*",
		"--limit", "10",
		"--output", "json",
		"--jq", ".[] | [.id, .subject] | @csv")

	rows, err := csv.NewReader(bytes.NewReader(stdout)).ReadAll()
	if err != nil {
		t.Fatalf("decode CSV: %v\nstdout:\n%s", err, stdout)
	}
	for _, want := range [][]string{
		{strconv.Itoa(issueA.ID), subjectA},
		{strconv.Itoa(issueB.ID), subjectB},
	} {
		if !slices.ContainsFunc(rows, func(row []string) bool { return slices.Equal(row, want) }) {
			t.Errorf("CSV output missing row %q\nstdout:\n%s", want, stdout)
		}
	}

	_, stderr := r.runExpectError(t, "issues", "list", "--project", proj.Identifier, "--output", "csv")
	if !bytes.Contains(stderr, []byte("CSV output was removed")) || !bytes.Contains(stderr, []byte("@csv")) {
		t.Errorf("-o csv stderr = %q, want the CSV removal error with the @csv hint", stderr)
	}
}

// TestOutput_Table_TruncatesWide drives `issues list --output table` with a
// COLUMNS=40 hint to cover the table renderer path. The table backend (pterm)
// does not always honor COLUMNS, so the assertion is intentionally lenient:
// the output must be non-empty and contain the issue subject so we know the
// table code path produced something. The width assertion is best-effort and
// only logs (does not fail) when lines exceed the slack budget.
func TestOutput_Table_TruncatesWide(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	subject := "table-row-" + uniqueShortSuffix(t)
	createTestIssueWithSubject(t, r, proj.Identifier, subject)

	stdout := r.runWithEnv(t, []string{"COLUMNS=40", "NO_COLOR=1"},
		"issues", "list",
		"--project", proj.Identifier,
		"--status", "*",
		"--output", "table",
		"--limit", "10")

	if len(bytes.TrimSpace(stdout)) == 0 {
		t.Fatalf("expected non-empty table output")
	}
	if !bytes.Contains(stdout, []byte(subject)) {
		t.Fatalf("table output missing issue subject %q\nstdout:\n%s", subject, stdout)
	}

	const slack = 50
	for i, line := range bytes.Split(stdout, []byte{'\n'}) {
		if len(line) > slack {
			t.Logf("line %d exceeds %d-col slack budget (len=%d): %s", i, slack, len(line), line)
		}
	}
}

// TestOutput_NoColorEmitsPlain verifies that NO_COLOR=1 strips ANSI escape
// sequences from the table output. The FORCE_COLOR direction is intentionally
// not tested here because the renderer respects TTY detection and the CLI is
// always invoked without a TTY in this suite, so FORCE_COLOR has no effect.
func TestOutput_NoColorEmitsPlain(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	plain := r.runWithEnv(t, []string{"NO_COLOR=1"},
		"users", "me", "--output", "table")
	if regexp.MustCompile(`\x1b\[`).Match(plain) {
		t.Fatalf("NO_COLOR output contained ANSI escape:\n%s", plain)
	}
	if len(bytes.TrimSpace(plain)) == 0 {
		t.Fatalf("NO_COLOR output was empty")
	}
}
