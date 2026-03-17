package group

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/resolver"
	"github.com/spf13/cobra"
)

func newCmdGroupRemoveUser(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remove-user <group-id-or-name> <user-id-or-name>",
		Short: "Remove a user from a group",
		Long:  "Remove a user from a group. Both arguments accept numeric IDs or names.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			groupID, err := resolver.ResolveGroup(context.Background(), client, args[0])
			if err != nil {
				return err
			}
			userID, err := resolver.ResolveUser(context.Background(), client, args[1])
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
