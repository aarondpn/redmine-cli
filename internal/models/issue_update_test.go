package models

import (
	"encoding/json"
	"testing"
)

func marshalIssueUpdate(t *testing.T, u IssueUpdate) map[string]any {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"issue": u})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out struct {
		Issue map[string]any `json:"issue"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal %s: %v", raw, err)
	}
	return out.Issue
}

func TestIssueUpdateMarshal_ZeroAssigneeSendsEmptyString(t *testing.T) {
	zero := 0
	notes := "unassigning"
	got := marshalIssueUpdate(t, IssueUpdate{AssignedToID: &zero, Notes: &notes})

	v, ok := got["assigned_to_id"]
	if !ok {
		t.Fatalf("assigned_to_id missing: %v", got)
	}
	if v != "" {
		t.Fatalf("assigned_to_id = %#v, want empty string", v)
	}
	if got["notes"] != notes {
		t.Fatalf("other fields must survive: %v", got)
	}
}

func TestIssueUpdateMarshal_PositiveAssigneeStaysNumeric(t *testing.T) {
	id := 12
	got := marshalIssueUpdate(t, IssueUpdate{AssignedToID: &id})
	if got["assigned_to_id"] != float64(12) {
		t.Fatalf("assigned_to_id = %#v, want 12", got["assigned_to_id"])
	}
}

func TestIssueUpdateMarshal_NilAssigneeOmitted(t *testing.T) {
	subject := "s"
	got := marshalIssueUpdate(t, IssueUpdate{Subject: &subject})
	if _, ok := got["assigned_to_id"]; ok {
		t.Fatalf("assigned_to_id should be omitted: %v", got)
	}
}
