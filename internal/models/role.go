package models

// Role represents a Redmine project role and its permissions.
type Role struct {
	ID                    int      `json:"id"`
	Name                  string   `json:"name"`
	Assignable            *bool    `json:"assignable,omitempty"`
	IssuesVisibility      string   `json:"issues_visibility,omitempty"`
	TimeEntriesVisibility string   `json:"time_entries_visibility,omitempty"`
	UsersVisibility       string   `json:"users_visibility,omitempty"`
	Permissions           []string `json:"permissions,omitzero"`
}

// IsAssignable reports whether the role is assignable when the API exposed
// that attribute.
func (r Role) IsAssignable() bool {
	return boolValue(r.Assignable)
}

func boolValue(v *bool) bool {
	return v != nil && *v
}
