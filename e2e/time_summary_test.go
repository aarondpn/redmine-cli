//go:build e2e

package e2e

import (
	"strconv"
	"testing"
	"time"
)

type summaryRow struct {
	Group string  `json:"group"`
	Hours float64 `json:"hours"`
}

// summaryDateWindow returns an 11-day window ending on the supplied anchor.
// Tests pass the same anchor to summaryDateWindow and the entry-date helpers
// so a midnight-UTC crossing during a test cannot shift the window relative
// to the entries it should contain.
func summaryDateWindow(anchor time.Time) (from, to string) {
	return anchor.AddDate(0, 0, -10).Format("2006-01-02"), anchor.Format("2006-01-02")
}

func allActivityNames(t *testing.T, r *cliRunner) []string {
	t.Helper()
	var resp struct {
		TimeEntryActivities []struct {
			Name string `json:"name"`
		} `json:"time_entry_activities"`
	}
	r.runJSON(t, &resp, "api", "/enumerations/time_entry_activities.json")
	names := make([]string, 0, len(resp.TimeEntryActivities))
	for _, a := range resp.TimeEntryActivities {
		names = append(names, a.Name)
	}
	return names
}

func logTimeEntry(t *testing.T, r *cliRunner, issueID int, hours float64, date, activity, comment string) {
	t.Helper()
	var created struct {
		Hours   float64 `json:"hours"`
		SpentOn string  `json:"spent_on"`
	}
	r.runJSON(t, &created, "time", "log",
		"--issue", strconv.Itoa(issueID),
		"--hours", strconv.FormatFloat(hours, 'f', -1, 64),
		"--activity", activity,
		"--date", date,
		"--comment", comment)
	if created.SpentOn != date {
		t.Fatalf("logged entry spent_on = %q, want %q", created.SpentOn, date)
	}
	if !approxEqual(created.Hours, hours) {
		t.Fatalf("logged entry hours = %v, want %v", created.Hours, hours)
	}
}

func findRow(rows []summaryRow, key string) *summaryRow {
	for i := range rows {
		if rows[i].Group == key {
			return &rows[i]
		}
	}
	return nil
}

// approxEqual tolerates float rounding from the JSON round-trip.
func approxEqual(a, b float64) bool {
	const eps = 1e-6
	d := a - b
	if d < 0 {
		d = -d
	}
	return d < eps
}

func TestTimeSummary_GroupByDay(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)
	activity := firstActivityName(t, r)

	anchor := time.Now().UTC()
	day1 := anchor.AddDate(0, 0, -4).Format("2006-01-02")
	day2 := anchor.AddDate(0, 0, -2).Format("2006-01-02")
	day3 := anchor.Format("2006-01-02")

	logTimeEntry(t, r, issue.ID, 1.0, day1, activity, "summary day1")
	logTimeEntry(t, r, issue.ID, 2.5, day2, activity, "summary day2")
	logTimeEntry(t, r, issue.ID, 0.75, day3, activity, "summary day3")

	from, to := summaryDateWindow(anchor)
	var rows []summaryRow
	r.runJSON(t, &rows, "time", "summary",
		"--from", from,
		"--to", to,
		"--group-by", "day",
		"--project", strconv.Itoa(proj.ID))

	if len(rows) != 3 {
		t.Fatalf("expected 3 day rows, got %d: %+v", len(rows), rows)
	}
	want := map[string]float64{day1: 1.0, day2: 2.5, day3: 0.75}
	for date, hours := range want {
		row := findRow(rows, date)
		if row == nil {
			t.Fatalf("missing summary row for %s; got %+v", date, rows)
		}
		if !approxEqual(row.Hours, hours) {
			t.Fatalf("row for %s hours = %v, want %v", date, row.Hours, hours)
		}
	}
}

func TestTimeSummary_GroupByActivity(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())

	activities := allActivityNames(t, r)
	if len(activities) < 2 {
		t.Skip("need >=2 time entry activities on the server")
	}
	first, second := activities[0], activities[1]

	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)

	anchor := time.Now().UTC()
	date := anchor.Format("2006-01-02")
	logTimeEntry(t, r, issue.ID, 1.5, date, first, "summary act1 a")
	logTimeEntry(t, r, issue.ID, 0.5, date, first, "summary act1 b")
	logTimeEntry(t, r, issue.ID, 2.25, date, second, "summary act2")

	from, to := summaryDateWindow(anchor)
	var rows []summaryRow
	r.runJSON(t, &rows, "time", "summary",
		"--from", from,
		"--to", to,
		"--group-by", "activity",
		"--project", strconv.Itoa(proj.ID))

	if len(rows) != 2 {
		t.Fatalf("expected 2 activity rows, got %d: %+v", len(rows), rows)
	}
	if row := findRow(rows, first); row == nil || !approxEqual(row.Hours, 2.0) {
		t.Fatalf("activity %q row = %+v, want hours=2.0", first, row)
	}
	if row := findRow(rows, second); row == nil || !approxEqual(row.Hours, 2.25) {
		t.Fatalf("activity %q row = %+v, want hours=2.25", second, row)
	}
}

func TestTimeSummary_GroupByProject(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	activity := firstActivityName(t, r)

	projA := createTestProject(t, r)
	issueA := createTestIssue(t, r, projA.Identifier)
	projB := createTestProject(t, r)
	issueB := createTestIssue(t, r, projB.Identifier)

	anchor := time.Now().UTC()
	date := anchor.Format("2006-01-02")
	logTimeEntry(t, r, issueA.ID, 1.0, date, activity, "summary projA a")
	logTimeEntry(t, r, issueA.ID, 0.5, date, activity, "summary projA b")
	logTimeEntry(t, r, issueB.ID, 3.25, date, activity, "summary projB")

	from, to := summaryDateWindow(anchor)

	var rowsA []summaryRow
	r.runJSON(t, &rowsA, "time", "summary",
		"--from", from,
		"--to", to,
		"--group-by", "project",
		"--project", strconv.Itoa(projA.ID))
	if len(rowsA) != 1 {
		t.Fatalf("project A summary expected 1 row, got %+v", rowsA)
	}
	if row := findRow(rowsA, projA.Name); row == nil || !approxEqual(row.Hours, 1.5) {
		t.Fatalf("project A row = %+v, want hours=1.5", row)
	}

	var rowsB []summaryRow
	r.runJSON(t, &rowsB, "time", "summary",
		"--from", from,
		"--to", to,
		"--group-by", "project",
		"--project", strconv.Itoa(projB.ID))
	if len(rowsB) != 1 {
		t.Fatalf("project B summary expected 1 row, got %+v", rowsB)
	}
	if row := findRow(rowsB, projB.Name); row == nil || !approxEqual(row.Hours, 3.25) {
		t.Fatalf("project B row = %+v, want hours=3.25", row)
	}
}

// TestTimeSummary_DateWindow logs entries on three dates and queries two
// narrow windows that bracket the middle date. The middle row must be absent
// from both windows and the two outer dates must carry their original hours.
func TestTimeSummary_DateWindow(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)
	activity := firstActivityName(t, r)

	anchor := time.Now().UTC()
	early := anchor.AddDate(0, 0, -8).Format("2006-01-02")
	middle := anchor.AddDate(0, 0, -5).Format("2006-01-02")
	late := anchor.AddDate(0, 0, -2).Format("2006-01-02")

	logTimeEntry(t, r, issue.ID, 1.0, early, activity, "summary early")
	logTimeEntry(t, r, issue.ID, 2.0, middle, activity, "summary middle")
	logTimeEntry(t, r, issue.ID, 3.0, late, activity, "summary late")

	leftFrom := anchor.AddDate(0, 0, -9).Format("2006-01-02")
	leftTo := anchor.AddDate(0, 0, -6).Format("2006-01-02")
	rightFrom := anchor.AddDate(0, 0, -3).Format("2006-01-02")
	rightTo := anchor.AddDate(0, 0, -1).Format("2006-01-02")

	var leftRows []summaryRow
	r.runJSON(t, &leftRows, "time", "summary",
		"--from", leftFrom,
		"--to", leftTo,
		"--group-by", "day",
		"--project", strconv.Itoa(proj.ID))
	if len(leftRows) != 1 || leftRows[0].Group != early || !approxEqual(leftRows[0].Hours, 1.0) {
		t.Fatalf("left window expected single early row, got %+v", leftRows)
	}
	if findRow(leftRows, middle) != nil {
		t.Fatalf("middle date %s leaked into left window: %+v", middle, leftRows)
	}

	var rightRows []summaryRow
	r.runJSON(t, &rightRows, "time", "summary",
		"--from", rightFrom,
		"--to", rightTo,
		"--group-by", "day",
		"--project", strconv.Itoa(proj.ID))
	if len(rightRows) != 1 || rightRows[0].Group != late || !approxEqual(rightRows[0].Hours, 3.0) {
		t.Fatalf("right window expected single late row, got %+v", rightRows)
	}
	if findRow(rightRows, middle) != nil {
		t.Fatalf("middle date %s leaked into right window: %+v", middle, rightRows)
	}
}

func TestTimeSummary_UserFilter(t *testing.T) {
	requireE2E(t)
	r := newCLIRunner(t, e2eBaseURL(), e2eAPIKey())
	proj := createTestProject(t, r)
	issue := createTestIssue(t, r, proj.Identifier)
	activity := firstActivityName(t, r)

	anchor := time.Now().UTC()
	date := anchor.Format("2006-01-02")
	logTimeEntry(t, r, issue.ID, 1.25, date, activity, "summary user me")

	from, to := summaryDateWindow(anchor)

	var meRows []summaryRow
	r.runJSON(t, &meRows, "time", "summary",
		"--from", from,
		"--to", to,
		"--group-by", "day",
		"--user", "me",
		"--project", strconv.Itoa(proj.ID))
	if len(meRows) != 1 || meRows[0].Group != date || !approxEqual(meRows[0].Hours, 1.25) {
		t.Fatalf("--user me expected single row for %s with 1.25 hours, got %+v", date, meRows)
	}

	var emptyRows []summaryRow
	r.runJSON(t, &emptyRows, "time", "summary",
		"--from", from,
		"--to", to,
		"--group-by", "day",
		"--user", "999999",
		"--project", strconv.Itoa(proj.ID))
	if len(emptyRows) != 0 {
		t.Fatalf("--user 999999 expected no rows, got %+v", emptyRows)
	}
}
