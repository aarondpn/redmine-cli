package models

// Role represents a Redmine project role and its permissions.
type Role struct {
	ID                    int      `json:"id"`
	Name                  string   `json:"name"`
	Assignable            *bool    `json:"assignable,omitempty"`
	Builtin               *bool    `json:"builtin,omitempty"`
	IsBuiltin             *bool    `json:"is_builtin,omitempty"`
	IssuesVisibility      string   `json:"issues_visibility,omitempty"`
	TimeEntriesVisibility string   `json:"time_entries_visibility,omitempty"`
	UsersVisibility       string   `json:"users_visibility,omitempty"`
	Permissions           []string `json:"permissions,omitempty"`
}

// IsBuiltIn reports whether the role is one of Redmine's built-in roles.
func (r Role) IsBuiltIn() bool {
	return boolValue(r.Builtin) || boolValue(r.IsBuiltin)
}

// IsAssignable reports whether the role is assignable when the API exposed
// that attribute.
func (r Role) IsAssignable() bool {
	return boolValue(r.Assignable)
}

func boolValue(v *bool) bool {
	return v != nil && *v
}
