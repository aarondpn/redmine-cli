package models

// IssueCategory represents a Redmine issue category.
type IssueCategory struct {
	ID         int     `json:"id"`
	Name       string  `json:"name"`
	Project    IDName  `json:"project"`
	AssignedTo *IDName `json:"assigned_to,omitempty"`
}

// Issue represents a Redmine issue.
type Issue struct {
	ID             int                `json:"id"`
	Project        IDName             `json:"project"`
	Tracker        IDName             `json:"tracker"`
	Status         IDName             `json:"status"`
	Priority       IDName             `json:"priority"`
	Author         IDName             `json:"author"`
	AssignedTo     *IDName            `json:"assigned_to,omitempty"`
	Category       *IDName            `json:"category,omitempty"`
	FixedVersion   *IDName            `json:"fixed_version,omitempty"`
	Parent         *IDRef             `json:"parent,omitempty"`
	Subject        string             `json:"subject"`
	Description    string             `json:"description"`
	DoneRatio      int                `json:"done_ratio"`
	IsPrivate      bool               `json:"is_private,omitempty"`
	CreatedOn      string             `json:"created_on"`
	UpdatedOn      string             `json:"updated_on"`
	StartDate      string             `json:"start_date,omitempty"`
	DueDate        string             `json:"due_date,omitempty"`
	EstimatedHours *float64           `json:"estimated_hours,omitempty"`
	CustomFields   []CustomFieldValue `json:"custom_fields,omitempty"`
	Journals       []Journal          `json:"journals,omitempty"`
	Children       []IDRef            `json:"children,omitempty"`
	Watchers       []IDName           `json:"watchers,omitempty"`
	Relations      []IssueRelation    `json:"relations,omitempty"`
}

// IssueRelation represents a relation between two issues. Returned both via
// /issues/{id}.json?include=relations and /issues/{id}/relations.json.
type IssueRelation struct {
	ID           int    `json:"id"`
	IssueID      int    `json:"issue_id"`
	IssueToID    int    `json:"issue_to_id"`
	RelationType string `json:"relation_type"`
	Delay        *int   `json:"delay,omitempty"`
}

// IssueRelationCreate defines the payload for creating an issue relation.
type IssueRelationCreate struct {
	IssueToID    int    `json:"issue_to_id"`
	RelationType string `json:"relation_type,omitempty"`
	Delay        *int   `json:"delay,omitempty"`
}

// IssueFilter defines parameters for listing issues.
type IssueFilter struct {
	ProjectID      string
	SubprojectID   string // "!*" excludes subprojects, numeric ID restricts to one
	TrackerID      int
	StatusID       string // "open", "closed", "*", or numeric ID
	AssignedToID   string // numeric ID or "me"
	AuthorID       string // numeric ID or "me"
	PriorityID     string // numeric ID
	CategoryID     int
	FixedVersionID int
	ParentID       int
	IsPrivate      *bool
	QueryID        int    // saved query ID (Redmine query_id parameter)
	Sort           string // e.g., "updated_on:desc"
	Includes       []string
	ExtraParams    map[string]string // escape hatch for Redmine filter syntax (e.g. created_on=">=2025-01-01", cf_5="Critical")
	Limit          int
	Offset         int
}

// Upload represents a file attachment reference for an issue create/update.
type Upload struct {
	Token       string `json:"token"`
	Filename    string `json:"filename"`
	ContentType string `json:"content_type,omitempty"`
	Description string `json:"description,omitempty"`
}

// IssueCreate defines fields for creating a new issue.
type IssueCreate struct {
	ProjectID         int               `json:"project_id"`
	TrackerID         int               `json:"tracker_id,omitempty"`
	StatusID          int               `json:"status_id,omitempty"`
	PriorityID        int               `json:"priority_id,omitempty"`
	Subject           string            `json:"subject"`
	Description       string            `json:"description,omitempty"`
	AssignedToID      int               `json:"assigned_to_id,omitempty"`
	ParentIssueID     int               `json:"parent_issue_id,omitempty"`
	CategoryID        int               `json:"category_id,omitempty"`
	FixedVersionID    int               `json:"fixed_version_id,omitempty"`
	StartDate         string            `json:"start_date,omitempty"`
	DueDate           string            `json:"due_date,omitempty"`
	EstimatedHours    float64           `json:"estimated_hours,omitempty"`
	IsPrivate         *bool             `json:"is_private,omitempty"`
	WatcherUserIDs    []int             `json:"watcher_user_ids,omitempty"`
	CustomFieldValues map[string]string `json:"custom_field_values,omitempty"`
	Uploads           []Upload          `json:"uploads,omitempty"`
}

// IssueUpdate defines fields for updating an issue. Nil fields are not sent.
type IssueUpdate struct {
	TrackerID         *int              `json:"tracker_id,omitempty"`
	StatusID          *int              `json:"status_id,omitempty"`
	PriorityID        *int              `json:"priority_id,omitempty"`
	Subject           *string           `json:"subject,omitempty"`
	Description       *string           `json:"description,omitempty"`
	AssignedToID      *int              `json:"assigned_to_id,omitempty"`
	DoneRatio         *int              `json:"done_ratio,omitempty"`
	Notes             *string           `json:"notes,omitempty"`
	PrivateNotes      *bool             `json:"private_notes,omitempty"`
	StartDate         *string           `json:"start_date,omitempty"`
	DueDate           *string           `json:"due_date,omitempty"`
	ParentIssueID     *int              `json:"parent_issue_id,omitempty"`
	CategoryID        *int              `json:"category_id,omitempty"`
	FixedVersionID    *int              `json:"fixed_version_id,omitempty"`
	EstimatedHours    *float64          `json:"estimated_hours,omitempty"`
	IsPrivate         *bool             `json:"is_private,omitempty"`
	CustomFieldValues map[string]string `json:"custom_field_values,omitempty"`
	Uploads           []Upload          `json:"uploads,omitempty"`
}
