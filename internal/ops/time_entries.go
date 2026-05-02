package ops

import (
	"context"
	"fmt"
	"sort"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

type ListTimeEntriesInput struct {
	ProjectID  string `json:"project_id,omitempty" jsonschema:"Project identifier or numeric ID to filter by."`
	UserID     string `json:"user_id,omitempty" jsonschema:"User numeric ID or 'me'."`
	IssueID    int    `json:"issue_id,omitempty" jsonschema:"Issue numeric ID to filter by."`
	ActivityID int    `json:"activity_id,omitempty" jsonschema:"Activity enumeration ID."`
	From       string `json:"from,omitempty" jsonschema:"Inclusive start date (YYYY-MM-DD)."`
	To         string `json:"to,omitempty" jsonschema:"Inclusive end date (YYYY-MM-DD)."`
	Limit      int    `json:"limit,omitempty" jsonschema:"Max results to return. Defaults to 50 when omitted."`
	Offset     int    `json:"offset,omitempty" jsonschema:"Number of leading results to skip."`
}

type TimeEntriesListResult struct {
	TimeEntries []models.TimeEntry `json:"time_entries"`
	Count       int                `json:"count"`
	TotalCount  int                `json:"total_count"`
}

type GetTimeEntryInput struct {
	ID int `json:"id" jsonschema:"Time entry numeric ID."`
}

type CreateTimeEntryInput struct {
	IssueID    int     `json:"issue_id,omitempty" jsonschema:"Issue to log time against. Either issue_id or project_id is required."`
	ProjectID  string  `json:"project_id,omitempty" jsonschema:"Project identifier or numeric ID. Either issue_id or project_id is required."`
	Hours      float64 `json:"hours" jsonschema:"Hours worked (decimal, e.g. 1.5)."`
	ActivityID int     `json:"activity_id,omitempty" jsonschema:"Activity enumeration ID."`
	SpentOn    string  `json:"spent_on,omitempty" jsonschema:"Date the work was done (YYYY-MM-DD). Defaults to today."`
	Comments   string  `json:"comments,omitempty" jsonschema:"Free-text comment."`
}

type UpdateTimeEntryInput struct {
	ID         int      `json:"id" jsonschema:"Time entry numeric ID."`
	Hours      *float64 `json:"hours,omitempty" jsonschema:"New hours worked."`
	ActivityID *int     `json:"activity_id,omitempty" jsonschema:"New activity enumeration ID."`
	SpentOn    *string  `json:"spent_on,omitempty" jsonschema:"New date (YYYY-MM-DD)."`
	Comments   *string  `json:"comments,omitempty" jsonschema:"New comment body."`
}

type DeleteTimeEntryInput struct {
	ID int `json:"id" jsonschema:"Time entry numeric ID."`
}

type SummaryTimeEntriesInput struct {
	ProjectID string `json:"project_id,omitempty" jsonschema:"Project identifier or numeric ID to filter by."`
	UserID    string `json:"user_id,omitempty" jsonschema:"User numeric ID or 'me'."`
	From      string `json:"from,omitempty" jsonschema:"Inclusive start date (YYYY-MM-DD)."`
	To        string `json:"to,omitempty" jsonschema:"Inclusive end date (YYYY-MM-DD)."`
	GroupBy   string `json:"group_by,omitempty" jsonschema:"One of 'day' (default), 'project', 'activity'."`
}

type TimeSummaryRow struct {
	Group string  `json:"group"`
	Hours float64 `json:"hours"`
}

type TimeSummaryResult struct {
	GroupBy    string           `json:"group_by"`
	From       string           `json:"from,omitempty"`
	To         string           `json:"to,omitempty"`
	Rows       []TimeSummaryRow `json:"rows"`
	TotalHours float64          `json:"total_hours"`
}

//mcpgen:tool list_time_entries
//mcpgen:description List Redmine time entries matching the given filters.
//mcpgen:category time
func ListTimeEntries(ctx context.Context, client *api.Client, input ListTimeEntriesInput) (TimeEntriesListResult, error) {
	entries, total, err := client.TimeEntries.List(ctx, models.TimeEntryFilter{
		ProjectID:  input.ProjectID,
		UserID:     input.UserID,
		IssueID:    input.IssueID,
		ActivityID: input.ActivityID,
		From:       input.From,
		To:         input.To,
		Limit:      ListLimit(input.Limit),
		Offset:     input.Offset,
	})
	if err != nil {
		return TimeEntriesListResult{}, err
	}
	return TimeEntriesListResult{TimeEntries: entries, Count: len(entries), TotalCount: total}, nil
}

//mcpgen:tool get_time_entry
//mcpgen:description Fetch a single time entry by ID.
//mcpgen:category time
func GetTimeEntry(ctx context.Context, client *api.Client, input GetTimeEntryInput) (*models.TimeEntry, error) {
	return client.TimeEntries.Get(ctx, input.ID)
}

//mcpgen:tool create_time_entry
//mcpgen:description Log a new time entry. Requires --enable-writes.
//mcpgen:category time
//mcpgen:writes
func CreateTimeEntry(ctx context.Context, client *api.Client, input CreateTimeEntryInput) (*models.TimeEntry, error) {
	if input.IssueID == 0 && input.ProjectID == "" {
		return nil, fmt.Errorf("either issue_id or project_id must be provided")
	}
	return client.TimeEntries.Create(ctx, models.TimeEntryCreate{
		IssueID:    input.IssueID,
		ProjectID:  input.ProjectID,
		Hours:      input.Hours,
		ActivityID: input.ActivityID,
		SpentOn:    input.SpentOn,
		Comments:   input.Comments,
	})
}

//mcpgen:tool update_time_entry
//mcpgen:description Update an existing time entry. Requires --enable-writes.
//mcpgen:category time
//mcpgen:writes
func UpdateTimeEntry(ctx context.Context, client *api.Client, input UpdateTimeEntryInput) (MessageResult, error) {
	err := client.TimeEntries.Update(ctx, input.ID, models.TimeEntryUpdate{
		Hours:      input.Hours,
		ActivityID: input.ActivityID,
		SpentOn:    input.SpentOn,
		Comments:   input.Comments,
	})
	if err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Updated time entry %d", input.ID)}, nil
}

//mcpgen:tool delete_time_entry
//mcpgen:description Delete a time entry. Destructive. Requires --enable-writes.
//mcpgen:category time
//mcpgen:writes
func DeleteTimeEntry(ctx context.Context, client *api.Client, input DeleteTimeEntryInput) (MessageResult, error) {
	if err := client.TimeEntries.Delete(ctx, input.ID); err != nil {
		return MessageResult{}, err
	}
	return MessageResult{Message: fmt.Sprintf("Deleted time entry %d", input.ID)}, nil
}

//mcpgen:tool summary_time_entries
//mcpgen:description Summarize time entries grouped by day, project, or activity.
//mcpgen:category time
func SummaryTimeEntries(ctx context.Context, client *api.Client, input SummaryTimeEntriesInput) (TimeSummaryResult, error) {
	groupBy := input.GroupBy
	if groupBy == "" {
		groupBy = "day"
	}
	if groupBy != "day" && groupBy != "project" && groupBy != "activity" {
		return TimeSummaryResult{}, fmt.Errorf("group_by must be one of 'day', 'project', or 'activity'")
	}

	entries, _, err := client.TimeEntries.List(ctx, models.TimeEntryFilter{
		ProjectID: input.ProjectID,
		UserID:    input.UserID,
		From:      input.From,
		To:        input.To,
		Limit:     0,
	})
	if err != nil {
		return TimeSummaryResult{}, err
	}

	totals := make(map[string]float64)
	for _, entry := range entries {
		var key string
		switch groupBy {
		case "project":
			key = entry.Project.Name
		case "activity":
			key = entry.Activity.Name
		default:
			key = entry.SpentOn
		}
		totals[key] += entry.Hours
	}

	keys := make([]string, 0, len(totals))
	for key := range totals {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	rows := make([]TimeSummaryRow, len(keys))
	var totalHours float64
	for i, key := range keys {
		rows[i] = TimeSummaryRow{Group: key, Hours: totals[key]}
		totalHours += totals[key]
	}

	return TimeSummaryResult{
		GroupBy:    groupBy,
		From:       input.From,
		To:         input.To,
		Rows:       rows,
		TotalHours: totalHours,
	}, nil
}

func GetTimeEntryForResource(ctx context.Context, client *api.Client, id int) (*models.TimeEntry, error) {
	return client.TimeEntries.Get(ctx, id)
}
