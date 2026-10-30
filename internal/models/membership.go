package models

// Membership represents a project membership. Exactly one of User, Group,
// GroupAnonymous and GroupNonMember is set: Redmine names the principal key
// after its class, and the two built-in groups have their own keys.
type Membership struct {
	ID             int              `json:"id"`
	Project        IDName           `json:"project"`
	User           *IDName          `json:"user,omitempty"`
	Group          *IDName          `json:"group,omitempty"`
	GroupAnonymous *IDName          `json:"group_anonymous,omitempty"`
	GroupNonMember *IDName          `json:"group_non_member,omitempty"`
	Roles          []MembershipRole `json:"roles"`
}

// MembershipRole is one role of a membership. Inherited is only sent (as true)
// for roles the principal holds through a group.
type MembershipRole struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Inherited bool   `json:"inherited,omitempty"`
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
