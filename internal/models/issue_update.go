package models

import "encoding/json"

// MarshalJSON encodes the update payload. A non-nil AssignedToID of 0 means
// "unassign": Redmine rejects assigned_to_id=0 ("Assignee is invalid") and
// only clears the assignee when it receives an empty string, so that case is
// sent as "assigned_to_id": "". Every other field keeps its struct encoding.
func (u IssueUpdate) MarshalJSON() ([]byte, error) {
	type plain IssueUpdate
	if u.AssignedToID == nil || *u.AssignedToID != 0 {
		return json.Marshal(plain(u))
	}
	// The outer AssignedToID shadows the embedded one (shallower depth wins
	// in encoding/json), replacing the integer with the empty string.
	return json.Marshal(struct {
		plain
		AssignedToID string `json:"assigned_to_id"`
	}{plain: plain(u)})
}
