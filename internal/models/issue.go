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
	CreatedOn      string             `json:"created_on"`
	UpdatedOn      string             `json:"updated_on"`
	StartDate      string             `json:"start_date,omitempty"`
	DueDate        string             `json:"due_date,omitempty"`
	EstimatedHours *float64           `json:"estimated_hours,omitempty"`
	CustomFields   []CustomFieldValue `json:"custom_fields,omitempty"`
	Journals       []Journal          `json:"journals,omitempty"`
	Children       []IDRef            `json:"children,omitempty"`
}

// IssueFilter defines parameters for listing issues.
type IssueFilter struct {
	ProjectID      string
	TrackerID      int
	StatusID       string // "open", "closed", "*", or numeric ID
	AssignedToID   string // numeric ID or "me"
	FixedVersionID int
	Sort           string // e.g., "updated_on:desc"
	Includes       []string
	Limit          int
	Offset         int
}

// IssueCreate defines fields for creating a new issue.
type IssueCreate struct {
	ProjectID      int     `json:"project_id"`
	TrackerID      int     `json:"tracker_id,omitempty"`
	StatusID       int     `json:"status_id,omitempty"`
	PriorityID     int     `json:"priority_id,omitempty"`
	Subject        string  `json:"subject"`
	Description    string  `json:"description,omitempty"`
	AssignedToID   int     `json:"assigned_to_id,omitempty"`
	ParentIssueID  int     `json:"parent_issue_id,omitempty"`
	CategoryID     int     `json:"category_id,omitempty"`
	FixedVersionID int     `json:"fixed_version_id,omitempty"`
	EstimatedHours float64 `json:"estimated_hours,omitempty"`
	IsPrivate      *bool   `json:"is_private,omitempty"`
}

// IssueUpdate defines fields for updating an issue. Nil fields are not sent.
type IssueUpdate struct {
	TrackerID      *int     `json:"tracker_id,omitempty"`
	StatusID       *int     `json:"status_id,omitempty"`
	PriorityID     *int     `json:"priority_id,omitempty"`
	Subject        *string  `json:"subject,omitempty"`
	Description    *string  `json:"description,omitempty"`
	AssignedToID   *int     `json:"assigned_to_id,omitempty"`
	DoneRatio      *int     `json:"done_ratio,omitempty"`
	Notes          *string  `json:"notes,omitempty"`
	DueDate        *string  `json:"due_date,omitempty"`
	ParentIssueID  *int     `json:"parent_issue_id,omitempty"`
	CategoryID     *int     `json:"category_id,omitempty"`
	FixedVersionID *int     `json:"fixed_version_id,omitempty"`
	EstimatedHours *float64 `json:"estimated_hours,omitempty"`
	IsPrivate      *bool    `json:"is_private,omitempty"`
}
