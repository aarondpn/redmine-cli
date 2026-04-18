package issue

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// NewCmdAssign creates the issues assign command.
func NewCmdAssign(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assign <id> <user-id-or-name>",
		Short: "Assign an issue to a user",
		Long:  "Assign an issue to a user. The user argument accepts a numeric ID, login, full name, or 'me'.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			userID, err := resolver.ResolveUser(context.Background(), client, args[1])
			if err != nil {
				return fmt.Errorf("resolving user: %w", err)
			}

			update := models.IssueUpdate{
				AssignedToID: &userID,
			}

			printer := f.Printer("")
			stop := printer.Spinner("Assigning issue...")
			err = client.Issues.Update(context.Background(), id, update)
			stop()
			if err != nil {
				return fmt.Errorf("failed to assign issue %s: %w", fmt.Sprintf("#%d", id), err)
			}

			printer.Action(output.ActionAssigned, "issue", id, fmt.Sprintf("Assigned issue #%d to user %d", id, userID))
			return nil
		},
	}

	return cmd
}
