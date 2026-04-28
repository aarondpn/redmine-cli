package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
)

func registerIssueTools(s *mcp.Server, client *api.Client, opts Options) {
	registerToolSpec(s, client, opts, toolSpec[ops.ListIssuesInput, ops.IssuesListResult]{
		Name:        "list_issues",
		Description: "List Redmine issues matching the given filters.",
		Call:        ops.ListIssues,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.GetIssueInput, *models.Issue]{
		Name:        "get_issue",
		Description: "Fetch a single Redmine issue by ID.",
		Call:        ops.GetIssue,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.CreateIssueInput, *models.Issue]{
		Name:        "create_issue",
		Description: "Create a new Redmine issue. Requires --enable-writes.",
		Writes:      true,
		Call:        ops.CreateIssue,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.UpdateIssueInput, ops.MessageResult]{
		Name:        "update_issue",
		Description: "Update fields on an existing Redmine issue. Requires --enable-writes.",
		Writes:      true,
		Call:        ops.UpdateIssue,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.DeleteIssueInput, ops.MessageResult]{
		Name:        "delete_issue",
		Description: "Delete a Redmine issue. Destructive. Requires --enable-writes.",
		Writes:      true,
		Call:        ops.DeleteIssue,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.AddIssueCommentInput, ops.MessageResult]{
		Name:        "add_issue_comment",
		Description: "Add a journal comment to an existing issue. Requires --enable-writes.",
		Writes:      true,
		Call:        ops.AddIssueComment,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.AssignIssueInput, ops.MessageResult]{
		Name:        "assign_issue",
		Description: "Assign an issue to a user. Requires --enable-writes.",
		Writes:      true,
		Call:        ops.AssignIssue,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.CloseIssueInput, ops.MessageResult]{
		Name:        "close_issue",
		Description: "Close an issue by setting its status to the first closed status. Requires --enable-writes.",
		Writes:      true,
		Call:        ops.CloseIssue,
	})
	registerToolSpec(s, client, opts, toolSpec[ops.ReopenIssueInput, ops.MessageResult]{
		Name:        "reopen_issue",
		Description: "Reopen a closed issue by setting its status to the first open status. Requires --enable-writes.",
		Writes:      true,
		Call:        ops.ReopenIssue,
	})
}
