package group

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
	"github.com/spf13/cobra"
)

func newCmdGroupAddUser(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-user <group-id-or-name> <user-id-or-name>",
		Short: "Add a user to a group",
		Long:  "Add a user to a group. Both arguments accept numeric IDs or names.",
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

			stop := printer.Spinner("Adding user to group...")
			_, err = ops.AddGroupUser(context.Background(), client, ops.GroupUserInput{
				GroupID: groupID,
				UserID:  userID,
			})
			stop()
			if err != nil {
				return err
			}

			printer.Action(output.ActionUserAdded, "group", groupID,
				fmt.Sprintf("Added user %d to group %d", userID, groupID))
			return nil
		},
	}

	return cmd
}
