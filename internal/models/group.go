package models

// Group represents a Redmine group.
type Group struct {
	ID          int              `json:"id"`
	Name        string           `json:"name"`
	Users       []IDName         `json:"users,omitzero"`
	Memberships []UserMembership `json:"memberships,omitzero"`
}

// GroupCreate defines fields for creating a group.
type GroupCreate struct {
	Name    string `json:"name"`
	UserIDs []int  `json:"user_ids,omitempty"`
}

// GroupUpdate defines fields for updating a group.
type GroupUpdate struct {
	Name    *string `json:"name,omitempty"`
	UserIDs *[]int  `json:"user_ids,omitempty"`
}

// GroupFilter defines parameters for listing groups.
type GroupFilter struct {
	Limit  int
	Offset int
}
