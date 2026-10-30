package models

import "encoding/json"

// IssueCategoryCreate is the payload for creating an issue category.
type IssueCategoryCreate struct {
	Name         string `json:"name"`
	AssignedToID int    `json:"assigned_to_id,omitempty"`
}

// IssueCategoryUpdate is the payload for updating an issue category. Nil
// fields are left unchanged.
type IssueCategoryUpdate struct {
	Name         *string `json:"name,omitempty"`
	AssignedToID *int    `json:"assigned_to_id,omitempty"`
}

// MarshalJSON encodes the update payload. A non-nil AssignedToID of 0 means
// "clear the default assignee", which Redmine only accepts as an empty string
// ("assigned_to_id": ""). See IssueUpdate.MarshalJSON.
func (u IssueCategoryUpdate) MarshalJSON() ([]byte, error) {
	type plain IssueCategoryUpdate
	if u.AssignedToID == nil || *u.AssignedToID != 0 {
		return json.Marshal(plain(u))
	}
	return json.Marshal(struct {
		plain
		AssignedToID string `json:"assigned_to_id"`
	}{plain: plain(u)})
}
