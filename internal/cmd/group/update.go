package group

import (
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
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

			id, err := resolver.ResolveGroup(cmdutil.Context(cmd), client, args[0])
			if err != nil {
				return err
			}

			printer := f.Printer("")

			input := ops.UpdateGroupInput{ID: id}
			if cmd.Flags().Changed("name") {
				input.Name = &name
			}
			if cmd.Flags().Changed("user-ids") {
				input.UserIDs = &userIDs
			}

			stop := printer.Spinner("Updating group...")
			_, err = ops.UpdateGroup(cmdutil.Context(cmd), client, input)
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
