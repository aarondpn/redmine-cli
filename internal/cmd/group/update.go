package group

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/spf13/cobra"
)

func newCmdGroupUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		name    string
		userIDs []int
	)

	cmd := &cobra.Command{
		Use:     "update <id>",
		Short:   "Update a group",
		Aliases: []string{"edit"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid group ID: %s", args[0])
			}

			client, err := f.ApiClient()
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

			printer.Success(fmt.Sprintf("Updated group %d", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Group name")
	cmd.Flags().IntSliceVar(&userIDs, "user-ids", nil, "User IDs (replaces current members)")
	return cmd
}
