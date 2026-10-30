package models

// IssueStatus represents a Redmine issue status.
type IssueStatus struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	IsClosed bool   `json:"is_closed"`

	// Description is only sent by Redmine 5.1+ (and never inside an issue's
	// allowed_statuses).
	Description *string `json:"description,omitempty"`
}
