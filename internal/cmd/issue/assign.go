package issue

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
)

// NewCmdAssign creates the issues assign command.
func NewCmdAssign(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "assign <id> <user_id>",
		Short: "Assign an issue to a user",
		Long:  "Assign an issue to a user by specifying the issue ID and user ID.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}

			userID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid user ID: %s", args[1])
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
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

			printer.Success(fmt.Sprintf("Assigned issue %s to user %d", fmt.Sprintf("#%d", id), userID))
			return nil
		},
	}

	return cmd
}
