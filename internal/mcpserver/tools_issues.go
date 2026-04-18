package mcpserver

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type listIssuesArgs struct {
	ProjectID      string   `json:"project_id,omitempty" jsonschema:"Project identifier or numeric ID to filter by."`
	TrackerID      int      `json:"tracker_id,omitempty" jsonschema:"Tracker ID (use list_trackers to discover)."`
	StatusID       string   `json:"status_id,omitempty" jsonschema:"Status filter: 'open', 'closed', '*', or a numeric status ID."`
	AssignedToID   string   `json:"assigned_to_id,omitempty" jsonschema:"Assignee: numeric user ID or 'me'."`
	FixedVersionID int      `json:"fixed_version_id,omitempty" jsonschema:"Fixed version (milestone) ID."`
	Sort           string   `json:"sort,omitempty" jsonschema:"Sort expression, e.g. 'updated_on:desc'."`
	Includes       []string `json:"includes,omitempty" jsonschema:"Extra fields to include: attachments, relations, children, watchers."`
	Limit          int      `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset         int      `json:"offset,omitempty" jsonschema:"Number of leading results to skip (pagination)."`
}

type issuesListResult struct {
	Issues     []models.Issue `json:"issues"`
	Count      int            `json:"count"`
	TotalCount int            `json:"total_count"`
}

type getIssueArgs struct {
	ID       int      `json:"id" jsonschema:"Numeric issue ID."`
	Includes []string `json:"includes,omitempty" jsonschema:"Extra sections to include: journals, attachments, relations, children, watchers."`
}

type createIssueArgs struct {
	ProjectID      int     `json:"project_id" jsonschema:"Numeric project ID to create the issue in."`
	Subject        string  `json:"subject" jsonschema:"Issue subject (title)."`
	Description    string  `json:"description,omitempty" jsonschema:"Issue body (Textile or Markdown depending on the Redmine configuration)."`
	TrackerID      int     `json:"tracker_id,omitempty" jsonschema:"Tracker ID (Bug, Feature, ...). Use list_trackers to discover."`
	StatusID       int     `json:"status_id,omitempty" jsonschema:"Initial status ID. Use list_statuses to discover."`
	PriorityID     int     `json:"priority_id,omitempty" jsonschema:"Priority ID."`
	AssignedToID   int     `json:"assigned_to_id,omitempty" jsonschema:"User ID of the assignee."`
	CategoryID     int     `json:"category_id,omitempty" jsonschema:"Issue category ID. Use list_categories to discover."`
	FixedVersionID int     `json:"fixed_version_id,omitempty" jsonschema:"Fixed version (milestone) ID."`
	ParentIssueID  int     `json:"parent_issue_id,omitempty" jsonschema:"Parent issue ID for sub-tasks."`
	EstimatedHours float64 `json:"estimated_hours,omitempty" jsonschema:"Estimated effort in hours."`
	IsPrivate      *bool   `json:"is_private,omitempty" jsonschema:"Mark the issue as private."`
}

type updateIssueArgs struct {
	ID             int      `json:"id" jsonschema:"Issue ID to update."`
	Subject        *string  `json:"subject,omitempty" jsonschema:"New subject (title)."`
	Description    *string  `json:"description,omitempty" jsonschema:"New description body."`
	TrackerID      *int     `json:"tracker_id,omitempty" jsonschema:"New tracker ID."`
	StatusID       *int     `json:"status_id,omitempty" jsonschema:"New status ID."`
	PriorityID     *int     `json:"priority_id,omitempty" jsonschema:"New priority ID."`
	AssignedToID   *int     `json:"assigned_to_id,omitempty" jsonschema:"User ID to assign. Pass 0 to unassign."`
	CategoryID     *int     `json:"category_id,omitempty" jsonschema:"Issue category ID."`
	FixedVersionID *int     `json:"fixed_version_id,omitempty" jsonschema:"Fixed version ID."`
	DoneRatio      *int     `json:"done_ratio,omitempty" jsonschema:"Completion percentage (0-100)."`
	EstimatedHours *float64 `json:"estimated_hours,omitempty" jsonschema:"Estimated effort in hours."`
	DueDate        *string  `json:"due_date,omitempty" jsonschema:"Due date (YYYY-MM-DD)."`
	Notes          *string  `json:"notes,omitempty" jsonschema:"Journal note to attach to the update."`
	IsPrivate      *bool    `json:"is_private,omitempty" jsonschema:"Toggle issue privacy."`
}

type deleteIssueArgs struct {
	ID int `json:"id" jsonschema:"Issue ID to delete."`
}

type addIssueCommentArgs struct {
	ID           int    `json:"id" jsonschema:"Issue ID to comment on."`
	Notes        string `json:"notes" jsonschema:"Comment body (journal note)."`
	PrivateNotes bool   `json:"private_notes,omitempty" jsonschema:"Mark the note as private."`
}

type assignIssueArgs struct {
	ID         int `json:"id" jsonschema:"Issue ID to assign."`
	AssigneeID int `json:"assignee_id" jsonschema:"User ID to assign. Must be > 0 (use update_issue to unassign)."`
}

type closeIssueArgs struct {
	ID    int    `json:"id" jsonschema:"Issue ID to close."`
	Notes string `json:"notes,omitempty" jsonschema:"Optional journal note to attach."`
}

type reopenIssueArgs struct {
	ID    int    `json:"id" jsonschema:"Issue ID to reopen."`
	Notes string `json:"notes,omitempty" jsonschema:"Optional journal note to attach."`
}

func registerIssueTools(s *mcp.Server, client *api.Client, opts Options) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_issues",
		Description: "List Redmine issues matching the given filters.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listIssuesArgs) (*mcp.CallToolResult, any, error) {
		filter := models.IssueFilter{
			ProjectID:      args.ProjectID,
			TrackerID:      args.TrackerID,
			StatusID:       args.StatusID,
			AssignedToID:   args.AssignedToID,
			FixedVersionID: args.FixedVersionID,
			Sort:           args.Sort,
			Includes:       args.Includes,
			Limit:          listLimit(args.Limit),
			Offset:         args.Offset,
		}
		issues, total, err := client.Issues.List(ctx, filter)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](issuesListResult{Issues: issues, Count: len(issues), TotalCount: total})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_issue",
		Description: "Fetch a single Redmine issue by ID.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getIssueArgs) (*mcp.CallToolResult, any, error) {
		issue, err := client.Issues.Get(ctx, args.ID, args.Includes)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](issue)
	})

	if !opts.EnableWrites {
		return
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_issue",
		Description: "Create a new Redmine issue. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createIssueArgs) (*mcp.CallToolResult, any, error) {
		issue, err := client.Issues.Create(ctx, models.IssueCreate{
			ProjectID:      args.ProjectID,
			Subject:        args.Subject,
			Description:    args.Description,
			TrackerID:      args.TrackerID,
			StatusID:       args.StatusID,
			PriorityID:     args.PriorityID,
			AssignedToID:   args.AssignedToID,
			CategoryID:     args.CategoryID,
			FixedVersionID: args.FixedVersionID,
			ParentIssueID:  args.ParentIssueID,
			EstimatedHours: args.EstimatedHours,
			IsPrivate:      args.IsPrivate,
		})
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](issue)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_issue",
		Description: "Update fields on an existing Redmine issue. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateIssueArgs) (*mcp.CallToolResult, any, error) {
		upd := models.IssueUpdate{
			Subject:        args.Subject,
			Description:    args.Description,
			TrackerID:      args.TrackerID,
			StatusID:       args.StatusID,
			PriorityID:     args.PriorityID,
			AssignedToID:   args.AssignedToID,
			CategoryID:     args.CategoryID,
			FixedVersionID: args.FixedVersionID,
			DoneRatio:      args.DoneRatio,
			EstimatedHours: args.EstimatedHours,
			DueDate:        args.DueDate,
			Notes:          args.Notes,
			IsPrivate:      args.IsPrivate,
		}
		if err := client.Issues.Update(ctx, args.ID, upd); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Updated issue #%d", args.ID))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_issue",
		Description: "Delete a Redmine issue. Destructive. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args deleteIssueArgs) (*mcp.CallToolResult, any, error) {
		if err := client.Issues.Delete(ctx, args.ID); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Deleted issue #%d", args.ID))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "add_issue_comment",
		Description: "Add a journal comment to an existing issue. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args addIssueCommentArgs) (*mcp.CallToolResult, any, error) {
		notes := args.Notes
		private := args.PrivateNotes
		upd := models.IssueUpdate{Notes: &notes, IsPrivate: &private}
		if err := client.Issues.Update(ctx, args.ID, upd); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Added comment to issue #%d", args.ID))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "assign_issue",
		Description: "Assign an issue to a user. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args assignIssueArgs) (*mcp.CallToolResult, any, error) {
		if args.AssigneeID <= 0 {
			return toolErr[any]("assignee_id must be a positive user ID; use update_issue to unassign")
		}
		id := args.AssigneeID
		upd := models.IssueUpdate{AssignedToID: &id}
		if err := client.Issues.Update(ctx, args.ID, upd); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Assigned issue #%d to user %d", args.ID, args.AssigneeID))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "close_issue",
		Description: "Close an issue by setting its status to the first closed status. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args closeIssueArgs) (*mcp.CallToolResult, any, error) {
		statusID, err := firstStatusID(ctx, client, true)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		upd := models.IssueUpdate{StatusID: &statusID}
		if args.Notes != "" {
			notes := args.Notes
			upd.Notes = &notes
		}
		if err := client.Issues.Update(ctx, args.ID, upd); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Closed issue #%d", args.ID))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "reopen_issue",
		Description: "Reopen a closed issue by setting its status to the first open status. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args reopenIssueArgs) (*mcp.CallToolResult, any, error) {
		statusID, err := firstStatusID(ctx, client, false)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		upd := models.IssueUpdate{StatusID: &statusID}
		if args.Notes != "" {
			notes := args.Notes
			upd.Notes = &notes
		}
		if err := client.Issues.Update(ctx, args.ID, upd); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Reopened issue #%d", args.ID))
	})
}

// firstStatusID returns the ID of the first status matching the desired
// closed flag. Mirrors the resolution pattern used by the close/reopen CLI
// commands.
func firstStatusID(ctx context.Context, client *api.Client, closed bool) (int, error) {
	statuses, err := client.Statuses.List(ctx)
	if err != nil {
		return 0, err
	}
	for _, s := range statuses {
		if s.IsClosed == closed {
			return s.ID, nil
		}
	}
	if closed {
		return 0, fmt.Errorf("no closed status found")
	}
	return 0, fmt.Errorf("no open status found")
}
