package models

// Membership represents a project membership.
type Membership struct {
	ID      int      `json:"id"`
	Project IDName   `json:"project"`
	User    *IDName  `json:"user,omitempty"`
	Group   *IDName  `json:"group,omitempty"`
	Roles   []IDName `json:"roles"`
}

// MembershipCreate defines fields for creating a membership.
type MembershipCreate struct {
	UserID  int   `json:"user_id"`
	RoleIDs []int `json:"role_ids"`
}

// MembershipUpdate defines fields for updating a membership.
type MembershipUpdate struct {
	RoleIDs []int `json:"role_ids"`
}

// MembershipFilter defines parameters for listing memberships.
type MembershipFilter struct {
	Project string
	Limit   int
	Offset  int
}
