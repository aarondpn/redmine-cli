package models

import (
	"encoding/json"
	"testing"
)

// roundTrip decodes a Redmine-shaped payload into v and re-encodes it, the
// same path every `-o json` and MCP tool response takes.
func roundTrip(t *testing.T, payload string, v any) map[string]json.RawMessage {
	t.Helper()
	if err := json.Unmarshal([]byte(payload), v); err != nil {
		t.Fatalf("decode: %v", err)
	}
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	var keys map[string]json.RawMessage
	if err := json.Unmarshal(out, &keys); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	return keys
}

func assertEmptyArray(t *testing.T, keys map[string]json.RawMessage, key string) {
	t.Helper()
	raw, ok := keys[key]
	if !ok {
		t.Errorf("%q missing from output, want []", key)
		return
	}
	if string(raw) != "[]" {
		t.Errorf("%q = %s, want []", key, raw)
	}
}

func assertAbsent(t *testing.T, keys map[string]json.RawMessage, key string) {
	t.Helper()
	if raw, ok := keys[key]; ok {
		t.Errorf("%q = %s, want key absent", key, raw)
	}
}

// Collections Redmine sends as empty arrays must survive as [], and
// collections it does not send must stay absent.
func TestResponseCollections_EmptyArraysRoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		v       any
		empty   []string
		absent  []string
	}{
		{
			name:    "issue",
			payload: `{"id":1,"journals":[],"attachments":[],"watchers":[]}`,
			v:       &Issue{},
			empty:   []string{"journals", "attachments", "watchers"},
			absent:  []string{"children", "relations", "custom_fields"},
		},
		{
			name:    "project",
			payload: `{"id":1,"trackers":[],"issue_categories":[],"enabled_modules":[],"time_entry_activities":[],"issue_custom_fields":[]}`,
			v:       &Project{},
			empty:   []string{"trackers", "issue_categories", "enabled_modules", "time_entry_activities", "issue_custom_fields"},
			absent:  []string{"custom_fields"},
		},
		{
			name:    "project without includes",
			payload: `{"id":1}`,
			v:       &Project{},
			absent:  []string{"trackers", "issue_categories", "enabled_modules", "time_entry_activities", "issue_custom_fields"},
		},
		{
			name:    "user",
			payload: `{"id":1,"memberships":[],"groups":[]}`,
			v:       &User{},
			empty:   []string{"memberships", "groups"},
			absent:  []string{"custom_fields"},
		},
		{
			name:    "user without includes",
			payload: `{"id":1}`,
			v:       &User{},
			absent:  []string{"memberships", "groups"},
		},
		{
			name:    "group",
			payload: `{"id":1,"users":[]}`,
			v:       &Group{},
			empty:   []string{"users"},
			absent:  []string{"memberships"},
		},
		{
			name:    "role",
			payload: `{"id":1,"permissions":[]}`,
			v:       &Role{},
			empty:   []string{"permissions"},
		},
		{
			name:    "role from list endpoint",
			payload: `{"id":1}`,
			v:       &Role{},
			absent:  []string{"permissions"},
		},
		{
			name:    "wiki page",
			payload: `{"title":"Home","attachments":[]}`,
			v:       &WikiPage{},
			empty:   []string{"attachments"},
		},
		{
			name:    "wiki page without includes",
			payload: `{"title":"Home"}`,
			v:       &WikiPage{},
			absent:  []string{"attachments"},
		},
		{
			name:    "journal",
			payload: `{"id":1,"details":[]}`,
			v:       &Journal{},
			empty:   []string{"details"},
		},
		{
			name:    "tracker",
			payload: `{"id":1,"enabled_standard_fields":[]}`,
			v:       &Tracker{},
			empty:   []string{"enabled_standard_fields"},
		},
		{
			name:    "custom field definition",
			payload: `{"id":1,"possible_values":[],"trackers":[],"roles":[]}`,
			v:       &CustomField{},
			empty:   []string{"possible_values", "trackers", "roles"},
			absent:  []string{"projects"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := roundTrip(t, tt.payload, tt.v)
			for _, k := range tt.empty {
				assertEmptyArray(t, keys, k)
			}
			for _, k := range tt.absent {
				assertAbsent(t, keys, k)
			}
		})
	}
}

// GET /issues/:id.json omits children for leaf issues and relations when
// there are none, even when both were requested.
func TestIssueNormalizeCollections_RequestedButOmitted(t *testing.T) {
	var issue Issue
	keys := roundTrip(t, `{"id":1}`, &issue)
	assertAbsent(t, keys, "children")

	issue.NormalizeCollections([]string{"children", " relations"})
	out, err := json.Marshal(issue)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	keys = nil
	if err := json.Unmarshal(out, &keys); err != nil {
		t.Fatalf("decode output: %v", err)
	}
	assertEmptyArray(t, keys, "children")
	assertEmptyArray(t, keys, "relations")
	assertEmptyArray(t, keys, "custom_fields")
	// Not requested: must not claim "none".
	assertAbsent(t, keys, "journals")
	assertAbsent(t, keys, "attachments")
	// Requested but withheld by Redmine (no view_issue_watchers permission):
	// stays absent rather than claiming there are no watchers.
	assertAbsent(t, keys, "watchers")
}

func TestHasInclude(t *testing.T) {
	tests := []struct {
		includes []string
		name     string
		want     bool
	}{
		{[]string{"children"}, "children", true},
		{[]string{" children "}, "children", true},
		{[]string{"journals", "children,relations"}, "relations", true},
		{[]string{"children, relations"}, "relations", true},
		{[]string{"journals,attachments"}, "children", false},
		{[]string{"childrens"}, "children", false},
		{nil, "children", false},
	}
	for _, tt := range tests {
		if got := hasInclude(tt.includes, tt.name); got != tt.want {
			t.Errorf("hasInclude(%q, %q) = %v, want %v", tt.includes, tt.name, got, tt.want)
		}
	}
}

// MCP clients may send includes as a single comma-joined entry.
func TestIssueNormalizeCollections_CommaJoinedIncludes(t *testing.T) {
	var issue Issue
	issue.NormalizeCollections([]string{"children,relations"})
	if issue.Children == nil || issue.Relations == nil {
		t.Fatalf("children/relations = %v/%v, want empty slices", issue.Children, issue.Relations)
	}
}

func TestIssueNormalizeCollections_NotRequested(t *testing.T) {
	issue := Issue{ID: 1}
	issue.NormalizeCollections(nil)
	if issue.Children != nil || issue.Relations != nil {
		t.Fatalf("children/relations = %v/%v, want nil when not requested", issue.Children, issue.Relations)
	}
	if issue.CustomFields == nil {
		t.Fatal("custom_fields = nil, want empty slice (always part of the issue response)")
	}
}

func TestIssueNormalizeCollections_KeepsData(t *testing.T) {
	issue := Issue{
		Children:     []IssueChild{{ID: 2}},
		Relations:    []IssueRelation{{ID: 3}},
		CustomFields: []CustomFieldValue{{ID: 4}},
	}
	issue.NormalizeCollections([]string{"children", "relations"})
	if len(issue.Children) != 1 || len(issue.Relations) != 1 || len(issue.CustomFields) != 1 {
		t.Fatalf("normalization dropped data: %+v", issue)
	}
}

func TestProjectAndUserNormalizeCollections(t *testing.T) {
	var p Project
	p.NormalizeCollections()
	keys := roundTrip(t, mustMarshal(t, p), &Project{})
	assertEmptyArray(t, keys, "custom_fields")
	assertAbsent(t, keys, "trackers")

	var u User
	u.NormalizeCollections()
	keys = roundTrip(t, mustMarshal(t, u), &User{})
	assertEmptyArray(t, keys, "custom_fields")
	assertAbsent(t, keys, "memberships")
	assertAbsent(t, keys, "groups")
}

// Request payloads must never gain empty arrays: Redmine treats e.g.
// `tracker_ids: []` or `user_ids: []` as "clear the field".
func TestRequestPayloads_OmitEmptyCollections(t *testing.T) {
	payloads := map[string]any{
		"IssueCreate":    IssueCreate{WatcherUserIDs: []int{}, Uploads: []Upload{}},
		"IssueUpdate":    IssueUpdate{Uploads: []Upload{}},
		"ProjectCreate":  ProjectCreate{TrackerIDs: []int{}, EnabledModuleNames: []string{}, IssueCustomFieldIDs: []int{}},
		"ProjectUpdate":  ProjectUpdate{TrackerIDs: []int{}, EnabledModuleNames: []string{}, IssueCustomFieldIDs: []int{}},
		"GroupCreate":    GroupCreate{UserIDs: []int{}},
		"GroupUpdate":    GroupUpdate{},
		"WikiPageCreate": WikiPageCreate{Uploads: []Upload{}},
		"WikiPageUpdate": WikiPageUpdate{Uploads: []Upload{}},
	}
	arrayKeys := []string{
		"watcher_user_ids", "uploads", "tracker_ids", "enabled_module_names",
		"issue_custom_field_ids", "user_ids", "custom_fields",
	}
	for name, v := range payloads {
		t.Run(name, func(t *testing.T) {
			var keys map[string]json.RawMessage
			if err := json.Unmarshal([]byte(mustMarshal(t, v)), &keys); err != nil {
				t.Fatalf("decode: %v", err)
			}
			for _, k := range arrayKeys {
				assertAbsent(t, keys, k)
			}
		})
	}
}

func mustMarshal(t *testing.T, v any) string {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return string(out)
}
