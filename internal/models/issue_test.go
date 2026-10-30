package models

import (
	"encoding/json"
	"reflect"
	"testing"
)

// issueShow70 mirrors issues/show.api.rsb on Redmine 7.0 with every include
// and permission granted: status.is_closed, the hour fields, closed_on, a
// three-level children tree (a leaf without "children", a node without
// "tracker"), changesets with and without a user, journals carrying
// updated_on/updated_by/reactions, allowed_statuses and issue reactions.
// Keys Redmine always sends but the models drop when empty (is_private,
// private_notes, null start_date/due_date) are left out on purpose so the
// lossless round-trip below holds; this is not the full wire payload.
const issueShow70 = `{
  "id": 10,
  "project": {"id": 1, "name": "P"},
  "tracker": {"id": 1, "name": "Bug"},
  "status": {"id": 5, "name": "Closed", "is_closed": true},
  "priority": {"id": 2, "name": "Normal"},
  "author": {"id": 1, "name": "Admin"},
  "subject": "Parent",
  "description": "",
  "done_ratio": 100,
  "created_on": "2026-01-01T10:00:00Z",
  "updated_on": "2026-01-03T10:00:00Z",
  "closed_on": "2026-01-03T10:00:00Z",
  "estimated_hours": 4.0,
  "total_estimated_hours": 6.5,
  "spent_hours": 0.0,
  "total_spent_hours": 2.25,
  "custom_fields": [],
  "children": [
    {"id": 11, "tracker": {"id": 1, "name": "Bug"}, "subject": "Child", "children": [
      {"id": 12, "tracker": {"id": 2, "name": "Feature"}, "subject": "Grandchild", "children": [
        {"id": 13, "subject": "No tracker leaf"}
      ]}
    ]},
    {"id": 14, "tracker": {"id": 1, "name": "Bug"}, "subject": "Leaf"}
  ],
  "changesets": [
    {"revision": "abc123", "user": {"id": 1, "name": "Admin"}, "comments": "fixes #10", "committed_on": "2026-01-02T09:00:00Z"},
    {"revision": "def456", "comments": "refs #10", "committed_on": "2026-01-02T10:00:00Z"}
  ],
  "journals": [
    {"id": 100, "user": {"id": 1, "name": "Admin"}, "notes": "edited", "created_on": "2026-01-02T00:00:00Z",
     "updated_on": "2026-01-02T01:00:00Z", "updated_by": {"id": 2, "name": "Other"},
     "details": [], "reactions": [{"user": {"id": 2, "name": "Other"}}]},
    {"id": 101, "user": {"id": 1, "name": "Admin"}, "notes": "plain", "created_on": "2026-01-02T02:00:00Z",
     "updated_on": "2026-01-02T02:00:00Z", "details": []}
  ],
  "allowed_statuses": [
    {"id": 1, "name": "New", "is_closed": false},
    {"id": 5, "name": "Closed", "is_closed": true}
  ],
  "reactions": [{"user": {"id": 1, "name": "Admin"}}]
}`

// issueShow42 mirrors issues/show.api.rsb on Redmine 4.2 for a caller without
// the view_time_entries permission: no is_closed, no spent_hours or
// total_spent_hours (gated by that permission), null estimated_hours and
// total_estimated_hours (always sent), no
// updated_on/updated_by on journals, and nothing for the 5.0+/7.0+ includes.
const issueShow42 = `{
  "id": 10,
  "project": {"id": 1, "name": "P"},
  "tracker": {"id": 1, "name": "Bug"},
  "status": {"id": 1, "name": "New"},
  "priority": {"id": 2, "name": "Normal"},
  "author": {"id": 1, "name": "Admin"},
  "subject": "Open",
  "description": "",
  "done_ratio": 0,
  "created_on": "2026-01-01T10:00:00Z",
  "updated_on": "2026-01-01T10:00:00Z",
  "closed_on": null,
  "estimated_hours": null,
  "total_estimated_hours": null,
  "journals": [
    {"id": 100, "user": {"id": 1, "name": "Admin"}, "notes": "hi", "created_on": "2026-01-02T00:00:00Z",
     "details": []}
  ]
}`

func decodeIssue(t *testing.T, payload string) Issue {
	t.Helper()
	var i Issue
	if err := json.Unmarshal([]byte(payload), &i); err != nil {
		t.Fatalf("decode: %v", err)
	}
	return i
}

func encodeToMap(t *testing.T, v any) map[string]any {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	return m
}

func TestIssue_Redmine70DecodesEveryField(t *testing.T) {
	i := decodeIssue(t, issueShow70)

	if i.Status.IsClosed == nil || !*i.Status.IsClosed {
		t.Errorf("Status.IsClosed = %v, want true", i.Status.IsClosed)
	}
	if i.ClosedOn != "2026-01-03T10:00:00Z" {
		t.Errorf("ClosedOn = %q", i.ClosedOn)
	}
	if i.TotalEstimatedHours == nil || *i.TotalEstimatedHours != 6.5 {
		t.Errorf("TotalEstimatedHours = %v", i.TotalEstimatedHours)
	}
	if i.SpentHours == nil || *i.SpentHours != 0 {
		t.Errorf("SpentHours = %v, want non-nil 0", i.SpentHours)
	}
	if i.TotalSpentHours == nil || *i.TotalSpentHours != 2.25 {
		t.Errorf("TotalSpentHours = %v", i.TotalSpentHours)
	}

	if len(i.Children) != 2 {
		t.Fatalf("Children = %d, want 2", len(i.Children))
	}
	grandchild := i.Children[0].Children[0]
	if grandchild.ID != 12 || grandchild.Tracker == nil || grandchild.Tracker.Name != "Feature" {
		t.Errorf("grandchild = %+v", grandchild)
	}
	deep := grandchild.Children[0]
	if deep.ID != 13 || deep.Tracker != nil || deep.Children != nil {
		t.Errorf("deepest node = %+v, want no tracker and no children", deep)
	}
	if leaf := i.Children[1]; leaf.Children != nil {
		t.Errorf("leaf Children = %v, want nil", leaf.Children)
	}

	if len(i.Changesets) != 2 {
		t.Fatalf("Changesets = %d, want 2", len(i.Changesets))
	}
	if cs := i.Changesets[0]; cs.Revision != "abc123" || cs.User == nil || cs.User.Name != "Admin" || cs.Comments != "fixes #10" || cs.CommittedOn == "" {
		t.Errorf("changeset 0 = %+v", cs)
	}
	if i.Changesets[1].User != nil {
		t.Errorf("changeset without user decoded User = %+v", i.Changesets[1].User)
	}

	if len(i.Journals) != 2 {
		t.Fatalf("Journals = %d, want 2", len(i.Journals))
	}
	edited := i.Journals[0]
	if edited.UpdatedOn != "2026-01-02T01:00:00Z" || edited.UpdatedBy == nil || edited.UpdatedBy.ID != 2 {
		t.Errorf("edited journal = %+v", edited)
	}
	if len(edited.Reactions) != 1 || edited.Reactions[0].User.Name != "Other" {
		t.Errorf("journal reactions = %+v", edited.Reactions)
	}
	if plain := i.Journals[1]; plain.UpdatedBy != nil || plain.Reactions != nil {
		t.Errorf("unedited journal = %+v, want no updated_by and no reactions", plain)
	}

	if len(i.AllowedStatuses) != 2 || i.AllowedStatuses[0].Name != "New" || !i.AllowedStatuses[1].IsClosed {
		t.Errorf("AllowedStatuses = %+v", i.AllowedStatuses)
	}
	if len(i.Reactions) != 1 || i.Reactions[0].User.ID != 1 {
		t.Errorf("Reactions = %+v", i.Reactions)
	}
}

// Everything Redmine 7.0 sent must come back out of a re-encode.
func TestIssue_Redmine70RoundTripLosesNothing(t *testing.T) {
	got := encodeToMap(t, decodeIssue(t, issueShow70))

	var want map[string]any
	if err := json.Unmarshal([]byte(issueShow70), &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		gj, _ := json.MarshalIndent(got, "", " ")
		wj, _ := json.MarshalIndent(want, "", " ")
		t.Errorf("round trip differs\ngot:\n%s\nwant:\n%s", gj, wj)
	}
}

// Keys that older Redmine versions do not send must stay out of the output
// instead of appearing as null, 0 or [].
func TestIssue_Redmine42KeepsGatedKeysAbsent(t *testing.T) {
	i := decodeIssue(t, issueShow42)
	if i.Status.IsClosed != nil {
		t.Errorf("Status.IsClosed = %v, want nil", *i.Status.IsClosed)
	}
	if i.ClosedOn != "" {
		t.Errorf("closed_on:null decoded to %q", i.ClosedOn)
	}

	got := encodeToMap(t, i)
	for _, k := range []string{
		"closed_on", "spent_hours", "total_spent_hours", "total_estimated_hours",
		"estimated_hours", "children", "changesets", "allowed_statuses", "reactions",
	} {
		if v, ok := got[k]; ok {
			t.Errorf("issue key %q = %v, want absent", k, v)
		}
	}
	status := got["status"].(map[string]any)
	if _, ok := status["is_closed"]; ok {
		t.Errorf("status.is_closed present: %v", status)
	}
	journal := got["journals"].([]any)[0].(map[string]any)
	for _, k := range []string{"updated_on", "updated_by", "reactions"} {
		if v, ok := journal[k]; ok {
			t.Errorf("journal key %q = %v, want absent", k, v)
		}
	}
}

// issues/index.api.rsb gained the hour sums and status.is_closed in 5.0; a
// 4.2 list omits them.
func TestIssue_IndexHourFieldsAreVersionGated(t *testing.T) {
	const idx51 = `{"id":1,"subject":"s","status":{"id":2,"name":"In Progress","is_closed":false},
		"estimated_hours":3.0,"total_estimated_hours":3.0,"spent_hours":1.5,"total_spent_hours":1.5,"closed_on":null}`
	const idx42 = `{"id":1,"subject":"s","status":{"id":2,"name":"In Progress"},"estimated_hours":3.0,"closed_on":null}`

	i51 := decodeIssue(t, idx51)
	if i51.Status.IsClosed == nil || *i51.Status.IsClosed {
		t.Errorf("5.1 IsClosed = %v, want non-nil false", i51.Status.IsClosed)
	}
	if i51.SpentHours == nil || *i51.SpentHours != 1.5 || i51.TotalSpentHours == nil || i51.TotalEstimatedHours == nil {
		t.Errorf("5.1 hours = %v %v %v", i51.SpentHours, i51.TotalSpentHours, i51.TotalEstimatedHours)
	}
	got51 := encodeToMap(t, i51)
	if got51["status"].(map[string]any)["is_closed"] != false {
		t.Errorf("5.1 status = %v, want is_closed false kept", got51["status"])
	}
	if got51["spent_hours"] != 1.5 {
		t.Errorf("5.1 spent_hours = %v", got51["spent_hours"])
	}
	if _, ok := got51["closed_on"]; ok {
		t.Error("closed_on:null must not be emitted")
	}

	got42 := encodeToMap(t, decodeIssue(t, idx42))
	for _, k := range []string{"spent_hours", "total_spent_hours", "total_estimated_hours"} {
		if _, ok := got42[k]; ok {
			t.Errorf("4.2 index key %q present", k)
		}
	}
	if _, ok := got42["status"].(map[string]any)["is_closed"]; ok {
		t.Error("4.2 status.is_closed present")
	}
}

func TestIssue_ZeroHoursAreKept(t *testing.T) {
	got := encodeToMap(t, decodeIssue(t, `{"id":1,"spent_hours":0.0,"total_spent_hours":0.0,"total_estimated_hours":0.0}`))
	for _, k := range []string{"spent_hours", "total_spent_hours", "total_estimated_hours"} {
		if v, ok := got[k]; !ok || v != float64(0) {
			t.Errorf("%q = %v (present=%v), want 0", k, v, ok)
		}
	}
}

func TestIssue_EmptyIncludedCollectionsRoundTrip(t *testing.T) {
	keys := roundTrip(t, `{"id":1,"changesets":[],"allowed_statuses":[],"reactions":[]}`, &Issue{})
	for _, k := range []string{"changesets", "allowed_statuses", "reactions"} {
		assertEmptyArray(t, keys, k)
	}
	assertAbsent(t, keys, "children")
}

func TestIssueStatusRef_IDName(t *testing.T) {
	closed := true
	got := IssueStatusRef{ID: 5, Name: "Closed", IsClosed: &closed}.IDName()
	if got != (IDName{ID: 5, Name: "Closed"}) {
		t.Errorf("IDName() = %+v", got)
	}
}
