package models

// Project represents a Redmine project.
type Project struct {
	ID          int      `json:"id"`
	Name        string   `json:"name"`
	Identifier  string   `json:"identifier"`
	Description string   `json:"description"`
	Status      int      `json:"status"`
	IsPublic    bool     `json:"is_public"`
	Parent      *IDName  `json:"parent,omitempty"`
	CreatedOn   string   `json:"created_on"`
	UpdatedOn   string   `json:"updated_on"`
	Trackers    []IDName `json:"trackers,omitempty"`
}

// ProjectCreate defines fields for creating a project.
type ProjectCreate struct {
	Name           string `json:"name"`
	Identifier     string `json:"identifier"`
	Description    string `json:"description,omitempty"`
	IsPublic       *bool  `json:"is_public,omitempty"`
	ParentID       int    `json:"parent_id,omitempty"`
	InheritMembers bool   `json:"inherit_members,omitempty"`
}

// ProjectUpdate defines fields for updating a project.
type ProjectUpdate struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
	IsPublic    *bool   `json:"is_public,omitempty"`
}

// Membership represents a project membership.
type Membership struct {
	ID      int      `json:"id"`
	Project IDName   `json:"project"`
	User    *IDName  `json:"user,omitempty"`
	Group   *IDName  `json:"group,omitempty"`
	Roles   []IDName `json:"roles"`
}
