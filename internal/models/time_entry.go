package models

// TimeEntry represents a Redmine time entry.
type TimeEntry struct {
	ID        int     `json:"id"`
	Project   IDName  `json:"project"`
	Issue     *IDRef  `json:"issue,omitempty"`
	User      IDName  `json:"user"`
	Activity  IDName  `json:"activity"`
	Hours     float64 `json:"hours"`
	Comments  string  `json:"comments"`
	SpentOn   string  `json:"spent_on"`
	CreatedOn string  `json:"created_on"`
	UpdatedOn string  `json:"updated_on"`
}

// TimeEntryCreate defines fields for creating a time entry.
type TimeEntryCreate struct {
	IssueID    int     `json:"issue_id,omitempty"`
	ProjectID  string  `json:"project_id,omitempty"`
	Hours      float64 `json:"hours"`
	ActivityID int     `json:"activity_id,omitempty"`
	SpentOn    string  `json:"spent_on,omitempty"`
	Comments   string  `json:"comments,omitempty"`
}

// TimeEntryUpdate defines fields for updating a time entry.
type TimeEntryUpdate struct {
	Hours      *float64 `json:"hours,omitempty"`
	ActivityID *int     `json:"activity_id,omitempty"`
	SpentOn    *string  `json:"spent_on,omitempty"`
	Comments   *string  `json:"comments,omitempty"`
}

// TimeEntryFilter defines parameters for listing time entries.
type TimeEntryFilter struct {
	ProjectID  string
	UserID     string
	IssueID    int
	From       string
	To         string
	ActivityID int
	Limit      int
	Offset     int
}
