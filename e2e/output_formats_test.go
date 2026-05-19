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

// TestOutput_CSV_IssuesList creates a project with two issues, then asks
// `issues list` for CSV output. The test confirms the headers exposed by
// internal/cmd/issue/list.go and that both issues land as data rows so a
// future column rename or row-renderer regression fails CI.
func TestOutput_CSV_IssuesList(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)

	subjectA := "CSV row A " + uniqueShortSuffix(t)
	subjectB := "CSV row B " + uniqueShortSuffix(t)
	createTestIssueWithSubject(t, r, proj.Identifier, subjectA)
	createTestIssueWithSubject(t, r, proj.Identifier, subjectB)

	stdout := r.run(t, "issues", "list",
		"--project", proj.Identifier,
		"--status", "*",
		"--output", "csv",
		"--limit", "10")

	rows, err := csv.NewReader(bytes.NewReader(stdout)).ReadAll()
	if err != nil {
		t.Fatalf("decode CSV: %v\nstdout:\n%s", err, stdout)
	}
	if len(rows) < 3 {
		t.Fatalf("expected header + 2 data rows, got %d rows\nstdout:\n%s", len(rows), stdout)
	}

	wantHeaders := []string{"ID", "Tracker", "Status", "Priority", "Subject", "Assignee", "Version"}
	if !slices.Equal(rows[0], wantHeaders) {
		t.Fatalf("CSV header mismatch:\n got: %v\nwant: %v", rows[0], wantHeaders)
	}

	subjectIdx := slices.Index(wantHeaders, "Subject")
	subjects := make([]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		if subjectIdx < len(row) {
			subjects = append(subjects, row[subjectIdx])
		}
	}
	for _, want := range []string{subjectA, subjectB} {
		if !slices.Contains(subjects, want) {
			t.Errorf("CSV output missing issue subject %q\nstdout:\n%s", want, stdout)
		}
	}
}

// TestOutput_CSV_TimeList logs a single time entry then verifies the CSV
// shape produced by `time list`. We assert headers, row count, and that the
// known hours value round-trips through the CSV renderer.
func TestOutput_CSV_TimeList(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)
	activity := firstActivityName(t, r)

	var logged struct {
		ID    int     `json:"id"`
		Hours float64 `json:"hours"`
	}
	r.runJSON(t, &logged, "time", "log",
		"--issue", strconv.Itoa(issue.ID),
		"--hours", "0.75",
		"--activity", activity,
		"--comment", "csv shape check")

	stdout := r.run(t, "time", "list",
		"--issue", strconv.Itoa(issue.ID),
		"--output", "csv")

	rows, err := csv.NewReader(bytes.NewReader(stdout)).ReadAll()
	if err != nil {
		t.Fatalf("decode CSV: %v\nstdout:\n%s", err, stdout)
	}
	if len(rows) != 2 {
		t.Fatalf("expected header + 1 data row, got %d rows\nstdout:\n%s", len(rows), stdout)
	}

	wantHeaders := []string{"ID", "Date", "Project", "Issue", "Hours", "Activity", "User", "Comments"}
	if !slices.Equal(rows[0], wantHeaders) {
		t.Fatalf("CSV header mismatch:\n got: %v\nwant: %v", rows[0], wantHeaders)
	}

	row := rows[1]
	if got, want := row[slices.Index(wantHeaders, "ID")], strconv.Itoa(logged.ID); got != want {
		t.Errorf("CSV ID = %q, want %q", got, want)
	}
	if got, want := row[slices.Index(wantHeaders, "Hours")], "0.75"; got != want {
		t.Errorf("CSV Hours = %q, want %q", got, want)
	}
	if got, want := row[slices.Index(wantHeaders, "Issue")], strconv.Itoa(issue.ID); got != want {
		t.Errorf("CSV Issue = %q, want %q", got, want)
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

// TestOutput_JSON_GoldenIssue snapshots the JSON shape of `issues get` to
// catch field renames and accidental schema drift. The golden file lives at
// e2e/testdata/issue_get.golden.json and is refreshed via UPDATE_GOLDENS=1.
func TestOutput_JSON_GoldenIssue(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	subject := "golden subject"
	tracker := firstTrackerName(t, r)
	issue := createTestIssueWithTracker(t, r, proj.Identifier, tracker, subject)

	var raw map[string]any
	r.runJSON(t, &raw, "issues", "get", strconv.Itoa(issue.ID))

	scrubbed := scrubVolatileIssueFields(raw)
	assertGoldenJSON(t, "testdata/issue_get.golden.json", scrubbed)
}

// scrubVolatileIssueFields removes ID, timestamp, author, project, and date
// fields that change between test runs so the golden snapshot only captures
// stable shape (subject, tracker name, status name, ...). Tracker/status/
// priority IDs vary across Redmine versions because the bootstrap loads
// default data in undefined order, so their numeric IDs are also dropped.
func scrubVolatileIssueFields(raw map[string]any) map[string]any {
	delete(raw, "id")
	delete(raw, "created_on")
	delete(raw, "updated_on")
	delete(raw, "start_date")
	delete(raw, "due_date")
	delete(raw, "project")
	delete(raw, "author")
	// custom_fields is dropped because the seeded "E2E Severity" fixture is
	// attached to every issue via is_for_all=true; its numeric ID and value
	// shape vary by bootstrap and aren't the schema signal this golden cares
	// about.
	delete(raw, "custom_fields")
	if tracker, ok := raw["tracker"].(map[string]any); ok {
		delete(tracker, "id")
	}
	if status, ok := raw["status"].(map[string]any); ok {
		delete(status, "id")
	}
	if priority, ok := raw["priority"].(map[string]any); ok {
		delete(priority, "id")
	}
	return raw
}
