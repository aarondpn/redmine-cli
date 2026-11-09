package issue

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

const fullIssueJSON = `{"id":7,"project":{"id":1,"name":"Proj"},"tracker":{"id":1,"name":"Bug"},
"status":{"id":2,"name":"In Progress"},"priority":{"id":3,"name":"High"},"author":{"id":1,"name":"Alice"},
"assigned_to":{"id":5,"name":"Bob"},"category":{"id":4,"name":"Backend"},"fixed_version":{"id":9,"name":"v1"},
"subject":"Crash","description":"it crashes","done_ratio":30,"is_private":true,
"created_on":"2026-01-01T00:00:00Z","updated_on":"2026-01-02T00:00:00Z","closed_on":"2026-01-03T00:00:00Z",
"estimated_hours":4,"total_estimated_hours":6,"spent_hours":1.5,"total_spent_hours":1.5,
"custom_fields":[{"id":3,"name":"Env","value":"prod"},{"id":4,"name":"Empty","value":""}],
"watchers":[{"id":5,"name":"Bob"},{"id":6,"name":"Carol"}],
"allowed_statuses":[{"id":1,"name":"New"},{"id":2,"name":"In Progress"}],
"children":[{"id":8,"tracker":{"id":1,"name":"Bug"},"subject":"child","children":[{"id":9,"subject":"grandchild"}]}],
"relations":[{"id":1,"issue_id":7,"issue_to_id":10,"relation_type":"blocks"},{"id":2,"issue_id":11,"issue_to_id":7,"relation_type":"precedes","delay":2}],
"changesets":[{"revision":"abc123","user":{"id":1,"name":"Alice"},"comments":"fix\nmore","committed_on":"2026-01-02"}],
"journals":[
{"id":1,"user":{"id":1,"name":"Alice"},"notes":"","created_on":"2026-01-01","details":[
 {"property":"attr","name":"status_id","old_value":"1","new_value":"2"},
 {"property":"attr","name":"assigned_to_id","old_value":"","new_value":"5"},
 {"property":"attr","name":"priority_id","old_value":"1","new_value":"3"},
 {"property":"cf","name":"3","old_value":"dev","new_value":"prod"},
 {"property":"attachment","name":"12","old_value":"","new_value":"a.png"},
 {"property":"attr","name":"description","old_value":"x","new_value":"y"}]},
{"id":2,"user":{"id":1,"name":"Alice"},"notes":"hello","created_on":"2026-01-02","updated_on":"2026-01-03","updated_by":{"id":6,"name":"Carol"},"private_notes":true},
{"id":3,"user":{"id":1,"name":"Alice"},"notes":"","created_on":"2026-01-04"}]}`

func render(t *testing.T, raw string) string {
	t.Helper()
	var is models.Issue
	if err := json.Unmarshal([]byte(raw), &is); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	writeIssueSections(&buf, &is)
	var rows strings.Builder
	for _, kv := range issueDetailRows(&is) {
		rows.WriteString(kv.Key + "=" + kv.Value + "\n")
	}
	return rows.String() + "====\n" + buf.String()
}

func TestIssueGetRender_Full(t *testing.T) {
	got := render(t, fullIssueJSON)
	for _, want := range []string{
		"Category=Backend\n",
		"Private=yes\n",
		"Closed=2026-01-03T00:00:00Z\n",
		"Estimated Hours=4.00 (total 6.00)\n",
		"Spent Hours=1.50\n",
		"Custom Fields=Env: prod\n",
		"Watchers=Bob, Carol\n",
		"Allowed Statuses=New, In Progress\n",
		"Children:\n  #8 [Bug] child\n    #9 grandchild\n",
		"Relations:\n  blocks #10\n  follows #11 (delay 2 days)\n",
		"Changesets:\n  abc123 Alice (2026-01-02) fix\n",
		"--- Alice (2026-01-01) [journal 1] ---\n* Status: New -> In Progress\n* Assignee: (none) -> Bob\n* Priority: 1 -> High\n* Env: dev -> prod\n* Attachment added: a.png\n* Description changed\n",
		"--- Alice (2026-01-02) (edited 2026-01-03 by Carol) (private) [journal 2] ---\nhello\n",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "journal 3") {
		t.Errorf("empty journal printed:\n%s", got)
	}
}

func TestIssueGetRender_Minimal(t *testing.T) {
	got := render(t, `{"id":1}`)
	for _, unwanted := range []string{"Category", "Private", "Closed", "Hours", "Custom", "Watchers", "Allowed", "Children", "Relations", "Changesets", "---"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("unexpected %q in:\n%s", unwanted, got)
		}
	}
}
