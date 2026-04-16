package group

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/output"
	"github.com/aarondpn/redmine-cli/internal/resolver"
	"github.com/spf13/cobra"
)

func newCmdGroupUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		name    string
		userIDs []int
	)

	cmd := &cobra.Command{
		Use:     "update <id-or-name>",
		Short:   "Update a group",
		Long:    "Update a group. Accepts a numeric ID or group name.",
		Aliases: []string{"edit"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			id, err := resolver.ResolveGroup(context.Background(), client, args[0])
			if err != nil {
				return err
			}

			printer := f.Printer("")

			update := models.GroupUpdate{}
			if cmd.Flags().Changed("name") {
				update.Name = &name
			}
			if cmd.Flags().Changed("user-ids") {
				update.UserIDs = &userIDs
			}

			stop := printer.Spinner("Updating group...")
			err = client.Groups.Update(context.Background(), id, update)
			stop()
			if err != nil {
				return err
			}

			printer.Action(output.ActionUpdated, "group", id, fmt.Sprintf("Updated group %d", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Group name")
	cmd.Flags().IntSliceVar(&userIDs, "user-ids", nil, "User IDs (replaces current members)")
	return cmd
}
