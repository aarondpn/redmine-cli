package ops

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type ListIssuesInput struct {
	ProjectID      string            `json:"project_id,omitempty" jsonschema:"Project identifier or numeric ID to filter by."`
	SubprojectID   string            `json:"subproject_id,omitempty" jsonschema:"Subproject filter: numeric ID to restrict to one subproject, '!*' to exclude subprojects entirely."`
	TrackerID      int               `json:"tracker_id,omitempty" jsonschema:"Tracker ID (use list_trackers to discover)."`
	StatusID       string            `json:"status_id,omitempty" jsonschema:"Status filter: 'open', 'closed', '*', or a numeric status ID."`
	AssignedToID   string            `json:"assigned_to_id,omitempty" jsonschema:"Assignee: numeric user ID or 'me'."`
	AuthorID       string            `json:"author_id,omitempty" jsonschema:"Author: numeric user ID or 'me'."`
	PriorityID     string            `json:"priority_id,omitempty" jsonschema:"Priority: numeric ID (use list_priorities to discover)."`
	CategoryID     int               `json:"category_id,omitempty" jsonschema:"Issue category ID (use list_categories to discover)."`
	FixedVersionID int               `json:"fixed_version_id,omitempty" jsonschema:"Fixed version (milestone) ID."`
	ParentID       int               `json:"parent_id,omitempty" jsonschema:"Restrict to children of this issue ID."`
	IsPrivate      *bool             `json:"is_private,omitempty" jsonschema:"Filter by privacy: true returns only private issues, false returns only public ones."`
	QueryID        int               `json:"query_id,omitempty" jsonschema:"Saved query ID. When set, the query's filters are applied server-side (use list_queries to discover)."`
	Sort           string            `json:"sort,omitempty" jsonschema:"Sort expression, e.g. 'updated_on:desc'."`
	Includes       []string          `json:"includes,omitempty" jsonschema:"Extra fields to include: attachments, relations, children, watchers."`
	ExtraParams    map[string]string `json:"extra_params,omitempty" jsonschema:"Raw Redmine filter parameters (escape hatch). Examples: {\"created_on\":\">=2025-01-01\",\"cf_5\":\"Critical\",\"subject\":\"~login\"}."`
	Limit          int               `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset         int               `json:"offset,omitempty" jsonschema:"Number of leading results to skip (pagination)."`
}

type IssuesListResult struct {
	Issues     []models.Issue `json:"issues"`
	Count      int            `json:"count"`
	TotalCount int            `json:"total_count"`
}

type GetIssueInput struct {
	ID       int      `json:"id" jsonschema:"Numeric issue ID."`
	Includes []string `json:"includes,omitempty" jsonschema:"Extra sections to include: journals, attachments, relations, children, watchers."`
}

type CreateIssueInput struct {
	ProjectID         int               `json:"project_id" jsonschema:"Numeric project ID to create the issue in."`
	Subject           string            `json:"subject" jsonschema:"Issue subject (title)."`
	Description       string            `json:"description,omitempty" jsonschema:"Issue body (Textile or Markdown depending on the Redmine configuration)."`
	TrackerID         int               `json:"tracker_id,omitempty" jsonschema:"Tracker ID (Bug, Feature, ...). Use list_trackers to discover."`
	StatusID          int               `json:"status_id,omitempty" jsonschema:"Initial status ID. Use list_statuses to discover."`
	PriorityID        int               `json:"priority_id,omitempty" jsonschema:"Priority ID."`
	AssignedToID      int               `json:"assigned_to_id,omitempty" jsonschema:"User ID of the assignee."`
	CategoryID        int               `json:"category_id,omitempty" jsonschema:"Issue category ID. Use list_categories to discover."`
	FixedVersionID    int               `json:"fixed_version_id,omitempty" jsonschema:"Fixed version (milestone) ID."`
	ParentIssueID     int               `json:"parent_issue_id,omitempty" jsonschema:"Parent issue ID for sub-tasks."`
	StartDate         string            `json:"start_date,omitempty" jsonschema:"Start date (YYYY-MM-DD)."`
	DueDate           string            `json:"due_date,omitempty" jsonschema:"Due date (YYYY-MM-DD)."`
	EstimatedHours    float64           `json:"estimated_hours,omitempty" jsonschema:"Estimated effort in hours."`
	IsPrivate         *bool             `json:"is_private,omitempty" jsonschema:"Mark the issue as private."`
	WatcherUserIDs    []int             `json:"watcher_user_ids,omitempty" jsonschema:"Initial watcher user IDs (numeric)."`
	CustomFieldValues map[string]string `json:"custom_field_values,omitempty" jsonschema:"Custom field values keyed by numeric custom-field ID. Use list_custom_fields to discover field IDs."`
	Uploads           []models.Upload   `json:"-"`
}

type UpdateIssueInput struct {
	ID                int               `json:"id" jsonschema:"Issue ID to update."`
	Subject           *string           `json:"subject,omitempty" jsonschema:"New subject (title)."`
	Description       *string           `json:"description,omitempty" jsonschema:"New description body."`
	TrackerID         *int              `json:"tracker_id,omitempty" jsonschema:"New tracker ID."`
	StatusID          *int              `json:"status_id,omitempty" jsonschema:"New status ID."`
	PriorityID        *int              `json:"priority_id,omitempty" jsonschema:"New priority ID."`
	AssignedToID      *int              `json:"assigned_to_id,omitempty" jsonschema:"Positive user ID to assign the issue to."`
	CategoryID        *int              `json:"category_id,omitempty" jsonschema:"Issue category ID."`
	FixedVersionID    *int              `json:"fixed_version_id,omitempty" jsonschema:"Fixed version ID."`
	ParentIssueID     *int              `json:"parent_issue_id,omitempty" jsonschema:"Parent issue ID for sub-tasks. Set to 0 to remove the parent."`
	DoneRatio         *int              `json:"done_ratio,omitempty" jsonschema:"Completion percentage (0-100)."`
	EstimatedHours    *float64          `json:"estimated_hours,omitempty" jsonschema:"Estimated effort in hours."`
	StartDate         *string           `json:"start_date,omitempty" jsonschema:"Start date (YYYY-MM-DD). Use empty string to clear."`
	DueDate           *string           `json:"due_date,omitempty" jsonschema:"Due date (YYYY-MM-DD). Use empty string to clear."`
	Notes             *string           `json:"notes,omitempty" jsonschema:"Journal note to attach to the update."`
	PrivateNotes      *bool             `json:"private_notes,omitempty" jsonschema:"Mark the note attached via 'notes' as private."`
	IsPrivate         *bool             `json:"is_private,omitempty" jsonschema:"Toggle issue privacy."`
	CustomFieldValues map[string]string `json:"custom_field_values,omitempty" jsonschema:"Custom field values keyed by numeric custom-field ID."`
	Uploads           []models.Upload   `json:"-"`
}

type DeleteIssueInput struct {
	ID int `json:"id" jsonschema:"Issue ID to delete."`
}

type AddIssueCommentInput struct {
	ID           int    `json:"id" jsonschema:"Issue ID to comment on."`
	Notes        string `json:"notes" jsonschema:"Comment body (journal note)."`
	PrivateNotes bool   `json:"private_notes,omitempty" jsonschema:"Mark the note as private."`
}

type AssignIssueInput struct {
	ID         int `json:"id" jsonschema:"Issue ID to assign."`
	AssigneeID int `json:"assignee_id" jsonschema:"User ID to assign. Must be > 0 (use update_issue to unassign)."`
}

type CloseIssueInput struct {
	ID    int    `json:"id" jsonschema:"Issue ID to close."`
	Notes string `json:"notes,omitempty" jsonschema:"Optional journal note to attach."`
}

type ReopenIssueInput struct {
	ID    int    `json:"id" jsonschema:"Issue ID to reopen."`
	Notes string `json:"notes,omitempty" jsonschema:"Optional journal note to attach."`
}

//mcpgen:tool list_issues
//mcpgen:description List Redmine issues matching the given filters.
//mcpgen:category issues
func ListIssues(ctx context.Context, client *api.Client, input ListIssuesInput) (IssuesListResult, error) {
	issues, total, err := client.Issues.List(ctx, models.IssueFilter{
		ProjectID:      input.ProjectID,
		SubprojectID:   input.SubprojectID,
		TrackerID:      input.TrackerID,
		StatusID:       input.StatusID,
		AssignedToID:   input.AssignedToID,
		AuthorID:       input.AuthorID,
		PriorityID:     input.PriorityID,
		CategoryID:     input.CategoryID,
		FixedVersionID: input.FixedVersionID,
		ParentID:       input.ParentID,
		IsPrivate:      input.IsPrivate,
		QueryID:        input.QueryID,
		Sort:           input.Sort,
		Includes:       input.Includes,
		ExtraParams:    input.ExtraParams,
		Limit:          ListLimit(input.Limit),
		Offset:         input.Offset,
	})
	if err != nil {
		return IssuesListResult{}, err
	}
	return IssuesListResult{Issues: issues, Count: len(issues), TotalCount: total}, nil
}

//mcpgen:tool get_issue
//mcpgen:description Fetch a single Redmine issue by ID.
//mcpgen:category issues
func GetIssue(ctx context.Context, client *api.Client, input GetIssueInput) (*models.Issue, error) {
	return client.Issues.Get(ctx, input.ID, input.Includes)
}

//mcpgen:tool create_issue
//mcpgen:description Create a new Redmine issue. Requires --enable-writes.
//mcpgen:category issues
//mcpgen:writes
func CreateIssue(ctx context.Context, client *api.Client, input CreateIssueInput) (*models.Issue, error) {
	return client.Issues.Create(ctx, models.IssueCreate{
		ProjectID:         input.ProjectID,
		Subject:           input.Subject,
		Description:       input.Description,
		TrackerID:         input.TrackerID,
		StatusID:          input.StatusID,
		PriorityID:        input.PriorityID,
		AssignedToID:      input.AssignedToID,
		CategoryID:        input.CategoryID,
		FixedVersionID:    input.FixedVersionID,
		ParentIssueID:     input.ParentIssueID,
		StartDate:         input.StartDate,
		DueDate:           input.DueDate,
		EstimatedHours:    input.EstimatedHours,
		IsPrivate:         input.IsPrivate,
		WatcherUserIDs:    input.WatcherUserIDs,
		CustomFieldValues: input.CustomFieldValues,
		Uploads:           input.Uploads,
	})
}

//mcpgen:tool update_issue
//mcpgen:description Update fields on an existing Redmine issue. Requires --enable-writes.
//mcpgen:category issues
//mcpgen:writes
func UpdateIssue(ctx context.Context, client *api.Client, input UpdateIssueInput) (MessageResult, error) {
	err := client.Issues.Update(ctx, input.ID, models.IssueUpdate{
		Subject:           input.Subject,
		Description:       input.Description,
		TrackerID:         input.TrackerID,
		StatusID:          input.StatusID,
		PriorityID:        input.PriorityID,
		AssignedToID:      input.AssignedToID,
		CategoryID:        input.CategoryID,
		FixedVersionID:    input.FixedVersionID,
		ParentIssueID:     input.ParentIssueID,
		DoneRatio:         input.DoneRatio,
		EstimatedHours:    input.EstimatedHours,
		StartDate:         input.StartDate,
		DueDate:           input.DueDate,
		Notes:             input.Notes,
		PrivateNotes:      input.PrivateNotes,
		IsPrivate:         input.IsPrivate,
		CustomFieldValues: input.CustomFieldValues,
		Uploads:           input.Uploads,
	})
	if err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Updated issue #%d", input.ID)}, nil
}

//mcpgen:tool delete_issue
//mcpgen:description Delete a Redmine issue. Destructive. Requires --enable-writes.
//mcpgen:category issues
//mcpgen:writes
func DeleteIssue(ctx context.Context, client *api.Client, input DeleteIssueInput) (MessageResult, error) {
	if err := client.Issues.Delete(ctx, input.ID); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Deleted issue #%d", input.ID)}, nil
}

//mcpgen:tool add_issue_comment
//mcpgen:description Add a journal comment to an existing issue. Requires --enable-writes.
//mcpgen:category issues
//mcpgen:writes
func AddIssueComment(ctx context.Context, client *api.Client, input AddIssueCommentInput) (MessageResult, error) {
	notes := input.Notes
	upd := models.IssueUpdate{Notes: &notes}
	if input.PrivateNotes {
		private := true
		upd.PrivateNotes = &private
	}
	if err := client.Issues.Update(ctx, input.ID, upd); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Added comment to issue #%d", input.ID)}, nil
}

func AssignIssue(ctx context.Context, client *api.Client, input AssignIssueInput) (MessageResult, error) {
	if input.AssigneeID <= 0 {
		return MessageResult{}, fmt.Errorf("assignee_id must be a positive user ID; use update_issue to unassign")
	}
	id := input.AssigneeID
	if err := client.Issues.Update(ctx, input.ID, models.IssueUpdate{AssignedToID: &id}); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Assigned issue #%d to user %d", input.ID, input.AssigneeID)}, nil
}

//mcpgen:tool close_issue
//mcpgen:description Close an issue by setting its status to the first closed status. Requires --enable-writes.
//mcpgen:category issues
//mcpgen:writes
func CloseIssue(ctx context.Context, client *api.Client, input CloseIssueInput) (MessageResult, error) {
	statusID, err := firstStatusID(ctx, client, true)
	if err != nil {
		return MessageResult{}, err
	}
	upd := models.IssueUpdate{StatusID: &statusID}
	if input.Notes != "" {
		notes := input.Notes
		upd.Notes = &notes
	}
	if err := client.Issues.Update(ctx, input.ID, upd); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Closed issue #%d", input.ID)}, nil
}

//mcpgen:tool reopen_issue
//mcpgen:description Reopen a closed issue by setting its status to the first open status. Requires --enable-writes.
//mcpgen:category issues
//mcpgen:writes
func ReopenIssue(ctx context.Context, client *api.Client, input ReopenIssueInput) (MessageResult, error) {
	statusID, err := firstStatusID(ctx, client, false)
	if err != nil {
		return MessageResult{}, err
	}
	upd := models.IssueUpdate{StatusID: &statusID}
	if input.Notes != "" {
		notes := input.Notes
		upd.Notes = &notes
	}
	if err := client.Issues.Update(ctx, input.ID, upd); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Reopened issue #%d", input.ID)}, nil
}

func GetIssueForResource(ctx context.Context, client *api.Client, id int) (*models.Issue, error) {
	return client.Issues.Get(ctx, id, []string{"journals", "attachments", "relations", "children", "watchers"})
}

type ListIssueWatchersInput struct {
	ID int `json:"id" jsonschema:"Issue ID."`
}

type IssueWatchersResult struct {
	Watchers []models.IDName `json:"watchers"`
	Count    int             `json:"count"`
}

type AddIssueWatcherInput struct {
	ID     int `json:"id" jsonschema:"Issue ID to add a watcher to."`
	UserID int `json:"user_id" jsonschema:"User ID to add as watcher (must be > 0)."`
}

type RemoveIssueWatcherInput struct {
	ID     int `json:"id" jsonschema:"Issue ID to remove a watcher from."`
	UserID int `json:"user_id" jsonschema:"User ID to remove (must be > 0)."`
}

//mcpgen:tool list_issue_watchers
//mcpgen:description List watchers of an issue.
//mcpgen:category issues
func ListIssueWatchers(ctx context.Context, client *api.Client, input ListIssueWatchersInput) (IssueWatchersResult, error) {
	watchers, err := client.Issues.ListWatchers(ctx, input.ID)
	if err != nil {
		return IssueWatchersResult{}, err
	}
	return IssueWatchersResult{Watchers: watchers, Count: len(watchers)}, nil
}

//mcpgen:tool add_issue_watcher
//mcpgen:description Add a user as a watcher on an issue. Requires --enable-writes.
//mcpgen:category issues
//mcpgen:writes
func AddIssueWatcher(ctx context.Context, client *api.Client, input AddIssueWatcherInput) (MessageResult, error) {
	if input.UserID <= 0 {
		return MessageResult{}, fmt.Errorf("user_id must be a positive user ID")
	}
	if err := client.Issues.AddWatcher(ctx, input.ID, input.UserID); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Added user %d as watcher of issue #%d", input.UserID, input.ID)}, nil
}

//mcpgen:tool remove_issue_watcher
//mcpgen:description Remove a user as a watcher from an issue. Requires --enable-writes.
//mcpgen:category issues
//mcpgen:writes
func RemoveIssueWatcher(ctx context.Context, client *api.Client, input RemoveIssueWatcherInput) (MessageResult, error) {
	if input.UserID <= 0 {
		return MessageResult{}, fmt.Errorf("user_id must be a positive user ID")
	}
	if err := client.Issues.RemoveWatcher(ctx, input.ID, input.UserID); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Removed user %d as watcher of issue #%d", input.UserID, input.ID)}, nil
}

func firstStatusID(ctx context.Context, client *api.Client, closed bool) (int, error) {
	statuses, err := client.Statuses.List(ctx)
	if err != nil {
		return 0, err
	}
	for _, status := range statuses {
		if status.IsClosed == closed {
			return status.ID, nil
		}
	}
	if closed {
		return 0, fmt.Errorf("no closed issue status is configured in Redmine")
	}
	return 0, fmt.Errorf("no open issue status is configured in Redmine")
}
