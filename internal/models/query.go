package models

// SavedQuery represents a Redmine saved query (custom filter) as returned by
// the /queries.json endpoint. Project queries carry a project_id; global
// queries omit it. The is_public flag indicates whether the query is visible
// to other users.
//
// ProjectID is a pointer because Redmine versions inconsistently encode the
// "global query" case: some omit the project_id key entirely, others emit
// `"project_id": null`. The pointer keeps both shapes distinguishable from
// project_id == 0 (which is not a value Redmine ever returns).
type SavedQuery struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	IsPublic  bool   `json:"is_public"`
	ProjectID *int   `json:"project_id,omitempty"`
}
