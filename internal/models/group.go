package models

// Group represents a Redmine group.
type Group struct {
	ID           int                `json:"id"`
	Name         string             `json:"name"`
	Builtin      string             `json:"builtin,omitempty"` // "anonymous" or "non_member", built-in groups only
	Users        []IDName           `json:"users,omitzero"`
	Memberships  []UserMembership   `json:"memberships,omitzero"`
	CustomFields []CustomFieldValue `json:"custom_fields,omitzero"`
}

// GroupCreate defines fields for creating a group.
type GroupCreate struct {
	Name              string            `json:"name"`
	UserIDs           []int             `json:"user_ids,omitempty"`
	TwofaRequired     *bool             `json:"twofa_required,omitempty"` // Redmine 5.0+
	CustomFieldValues CustomFieldValues `json:"custom_field_values,omitempty"`
}

// GroupUpdate defines fields for updating a group.
type GroupUpdate struct {
	Name              *string           `json:"name,omitempty"`
	UserIDs           *[]int            `json:"user_ids,omitempty"`
	TwofaRequired     *bool             `json:"twofa_required,omitempty"` // Redmine 5.0+
	CustomFieldValues CustomFieldValues `json:"custom_field_values,omitempty"`
}

// GroupFilter defines parameters for listing groups.
type GroupFilter struct {
	// Builtin also returns the builtin Anonymous and Non member groups
	// (builtin=1).
	Builtin bool
	Limit   int
	Offset  int
}
