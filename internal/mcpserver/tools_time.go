package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
)

func registerTimeEntryTools(s *mcp.Server, client *api.Client, opts Options) {
	registerToolSpec(s, client, opts, toolSpec[ops.ListTimeEntriesInput, ops.TimeEntriesListResult]{
		Name:        "list_time_entries",
		Description: "List time entries matching the given filters.",
		Call:        ops.ListTimeEntries,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.GetTimeEntryInput, *models.TimeEntry]{
		Name:        "get_time_entry",
		Description: "Fetch a single time entry by ID.",
		Call:        ops.GetTimeEntry,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.SummaryTimeEntriesInput, ops.TimeSummaryResult]{
		Name:        "summary_time_entries",
		Description: "Aggregate time entries grouped by day, project, or activity.",
		Call:        ops.SummaryTimeEntries,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.CreateTimeEntryInput, *models.TimeEntry]{
		Name:        "create_time_entry",
		Description: "Log time against an issue or project. Requires --enable-writes.",
		Writes:      true,
		Call:        ops.CreateTimeEntry,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.UpdateTimeEntryInput, ops.MessageResult]{
		Name:        "update_time_entry",
		Description: "Update an existing time entry. Requires --enable-writes.",
		Writes:      true,
		Call:        ops.UpdateTimeEntry,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.DeleteTimeEntryInput, ops.MessageResult]{
		Name:        "delete_time_entry",
		Description: "Delete a time entry. Destructive. Requires --enable-writes.",
		Writes:      true,
		Call:        ops.DeleteTimeEntry,
	})
}
