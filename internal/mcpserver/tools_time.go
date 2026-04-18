package mcpserver

import (
	"context"
	"fmt"
	"sort"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type listTimeEntriesArgs struct {
	ProjectID  string `json:"project_id,omitempty" jsonschema:"Project identifier or numeric ID to filter by."`
	UserID     string `json:"user_id,omitempty" jsonschema:"User numeric ID or 'me'."`
	IssueID    int    `json:"issue_id,omitempty" jsonschema:"Issue numeric ID to filter by."`
	ActivityID int    `json:"activity_id,omitempty" jsonschema:"Activity enumeration ID."`
	From       string `json:"from,omitempty" jsonschema:"Inclusive start date (YYYY-MM-DD)."`
	To         string `json:"to,omitempty" jsonschema:"Inclusive end date (YYYY-MM-DD)."`
	Limit      int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset     int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type timeEntriesListResult struct {
	TimeEntries []models.TimeEntry `json:"time_entries"`
	Count       int                `json:"count"`
	TotalCount  int                `json:"total_count"`
}

type getTimeEntryArgs struct {
	ID int `json:"id" jsonschema:"Time entry numeric ID."`
}

type createTimeEntryArgs struct {
	IssueID    int     `json:"issue_id,omitempty" jsonschema:"Issue to log time against. Either issue_id or project_id is required."`
	ProjectID  string  `json:"project_id,omitempty" jsonschema:"Project identifier or numeric ID. Either issue_id or project_id is required."`
	Hours      float64 `json:"hours" jsonschema:"Hours worked (decimal, e.g. 1.5)."`
	ActivityID int     `json:"activity_id,omitempty" jsonschema:"Activity enumeration ID."`
	SpentOn    string  `json:"spent_on,omitempty" jsonschema:"Date the work was done (YYYY-MM-DD). Defaults to today."`
	Comments   string  `json:"comments,omitempty" jsonschema:"Free-text comment."`
}

type updateTimeEntryArgs struct {
	ID         int      `json:"id" jsonschema:"Time entry numeric ID."`
	Hours      *float64 `json:"hours,omitempty" jsonschema:"New hours worked."`
	ActivityID *int     `json:"activity_id,omitempty" jsonschema:"New activity enumeration ID."`
	SpentOn    *string  `json:"spent_on,omitempty" jsonschema:"New date (YYYY-MM-DD)."`
	Comments   *string  `json:"comments,omitempty" jsonschema:"New comment body."`
}

type deleteTimeEntryArgs struct {
	ID int `json:"id" jsonschema:"Time entry numeric ID."`
}

type summaryTimeEntriesArgs struct {
	ProjectID string `json:"project_id,omitempty" jsonschema:"Project identifier or numeric ID to filter by."`
	UserID    string `json:"user_id,omitempty" jsonschema:"User numeric ID or 'me'."`
	From      string `json:"from,omitempty" jsonschema:"Inclusive start date (YYYY-MM-DD)."`
	To        string `json:"to,omitempty" jsonschema:"Inclusive end date (YYYY-MM-DD)."`
	GroupBy   string `json:"group_by,omitempty" jsonschema:"One of 'day' (default), 'project', 'activity'."`
}

type timeSummaryRow struct {
	Group string  `json:"group"`
	Hours float64 `json:"hours"`
}

type timeSummaryResult struct {
	GroupBy    string           `json:"group_by"`
	From       string           `json:"from,omitempty"`
	To         string           `json:"to,omitempty"`
	Rows       []timeSummaryRow `json:"rows"`
	TotalHours float64          `json:"total_hours"`
}

func registerTimeEntryTools(s *mcp.Server, client *api.Client, opts Options) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "list_time_entries",
		Description: "List time entries matching the given filters.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args listTimeEntriesArgs) (*mcp.CallToolResult, any, error) {
		entries, total, err := client.TimeEntries.List(ctx, models.TimeEntryFilter{
			ProjectID:  args.ProjectID,
			UserID:     args.UserID,
			IssueID:    args.IssueID,
			ActivityID: args.ActivityID,
			From:       args.From,
			To:         args.To,
			Limit:      listLimit(args.Limit),
			Offset:     args.Offset,
		})
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](timeEntriesListResult{TimeEntries: entries, Count: len(entries), TotalCount: total})
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_time_entry",
		Description: "Fetch a single time entry by ID.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args getTimeEntryArgs) (*mcp.CallToolResult, any, error) {
		entry, err := client.TimeEntries.Get(ctx, args.ID)
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](entry)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "summary_time_entries",
		Description: "Aggregate time entries grouped by day, project, or activity.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args summaryTimeEntriesArgs) (*mcp.CallToolResult, any, error) {
		groupBy := args.GroupBy
		if groupBy == "" {
			groupBy = "day"
		}
		entries, _, err := client.TimeEntries.List(ctx, models.TimeEntryFilter{
			ProjectID: args.ProjectID,
			UserID:    args.UserID,
			From:      args.From,
			To:        args.To,
			Limit:     0,
		})
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		totals := make(map[string]float64)
		for _, e := range entries {
			var key string
			switch groupBy {
			case "project":
				key = e.Project.Name
			case "activity":
				key = e.Activity.Name
			default:
				key = e.SpentOn
			}
			totals[key] += e.Hours
		}
		keys := make([]string, 0, len(totals))
		for k := range totals {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		rows := make([]timeSummaryRow, len(keys))
		var grand float64
		for i, k := range keys {
			rows[i] = timeSummaryRow{Group: k, Hours: totals[k]}
			grand += totals[k]
		}
		return toolOK[any](timeSummaryResult{
			GroupBy: groupBy, From: args.From, To: args.To, Rows: rows, TotalHours: grand,
		})
	})

	if !opts.EnableWrites {
		return
	}

	mcp.AddTool(s, &mcp.Tool{
		Name:        "create_time_entry",
		Description: "Log time against an issue or project. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args createTimeEntryArgs) (*mcp.CallToolResult, any, error) {
		if args.IssueID == 0 && args.ProjectID == "" {
			return toolErr[any]("either issue_id or project_id must be provided")
		}
		entry, err := client.TimeEntries.Create(ctx, models.TimeEntryCreate{
			IssueID:    args.IssueID,
			ProjectID:  args.ProjectID,
			Hours:      args.Hours,
			ActivityID: args.ActivityID,
			SpentOn:    args.SpentOn,
			Comments:   args.Comments,
		})
		if err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOK[any](entry)
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "update_time_entry",
		Description: "Update an existing time entry. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args updateTimeEntryArgs) (*mcp.CallToolResult, any, error) {
		if err := client.TimeEntries.Update(ctx, args.ID, models.TimeEntryUpdate{
			Hours:      args.Hours,
			ActivityID: args.ActivityID,
			SpentOn:    args.SpentOn,
			Comments:   args.Comments,
		}); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Updated time entry %d", args.ID))
	})

	mcp.AddTool(s, &mcp.Tool{
		Name:        "delete_time_entry",
		Description: "Delete a time entry. Destructive. Requires --enable-writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, args deleteTimeEntryArgs) (*mcp.CallToolResult, any, error) {
		if err := client.TimeEntries.Delete(ctx, args.ID); err != nil {
			return toolErrFromAPI[any](err)
		}
		return toolOKMsg(fmt.Sprintf("Deleted time entry %d", args.ID))
	})
}
