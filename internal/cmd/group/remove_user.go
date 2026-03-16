package group

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func newCmdGroupRemoveUser(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-user <group-id> <user-id>",
		Short: "Remove a user from a group",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			groupID, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid group ID: %s", args[0])
			}
			userID, err := strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid user ID: %s", args[1])
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")

			stop := printer.Spinner("Removing user from group...")
			err = client.Groups.RemoveUser(context.Background(), groupID, userID)
			stop()
			if err != nil {
				return err
			}

			printer.Success(fmt.Sprintf("Removed user %d from group %d", userID, groupID))
			return nil
		},
	}

	return cmd
}
