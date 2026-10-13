package issue

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// NewCmdAssign creates the issues assign command.
func NewCmdAssign(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assign <id> <assignee>",
		Short: "Assign an issue to a user or group",
		Long: `Assign an issue to a user or group, or clear its assignee.

The assignee accepts a numeric ID, 'me', a display name, a login, or 'none'
to unassign. Names are matched against the issue's project members (users and
groups), which works without admin privileges; logins and users outside the
project fall back to the admin-only users list. Assigning a group requires
Redmine's "Allow issue assignment to groups" setting.`,
		Example: `  # Assign to yourself
  redmine issues assign 123 me

  # Assign to a group or user by name
  redmine issues assign 123 "Backend Team"

  # Clear the assignee
  redmine issues assign 123 none`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			printer := f.Printer("")

			if resolver.IsNone(args[1]) {
				unassigned := 0
				stop := printer.Spinner("Unassigning issue...")
				_, err = ops.UpdateIssue(ctx, client, ops.UpdateIssueInput{ID: id, AssignedToID: &unassigned})
				stop()
				if err != nil {
					return fmt.Errorf("failed to unassign issue %s: %w", fmt.Sprintf("#%d", id), err)
				}
				printer.Action(output.ActionUnassigned, "issue", id, fmt.Sprintf("Unassigned issue #%d", id))
				return nil
			}

			userID, err := resolveIssuePrincipal(ctx, client, id, args[1])
			if err != nil {
				return fmt.Errorf("resolving user: %w", err)
			}

			stop := printer.Spinner("Assigning issue...")
			_, err = ops.AssignIssue(ctx, client, ops.AssignIssueInput{
				ID:         id,
				AssigneeID: userID,
			})
			stop()
			if err != nil {
				return fmt.Errorf("failed to assign issue %s: %w", fmt.Sprintf("#%d", id), err)
			}

			printer.Action(output.ActionAssigned, "issue", id, fmt.Sprintf("Assigned issue #%d to user %d", id, userID))
			return nil
		},
	}

	cmd.ValidArgsFunction = func(c *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) == 1 {
			return cmdutil.CompleteAssignees(f)(c, args, toComplete)
		}
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return cmd
}
