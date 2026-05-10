package models

// SavedQuery represents a Redmine saved query (custom filter) as returned by
// the /queries.json endpoint. Project queries carry a project_id; global
// queries omit it. The is_public flag indicates whether the query is visible
// to other users.
type SavedQuery struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	IsPublic  bool   `json:"is_public"`
	ProjectID *int   `json:"project_id,omitempty"`
}
