package group

import (
	"context"
	"fmt"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/spf13/cobra"
)

func newCmdGroupCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		name    string
		userIDs []int
		format  string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Short:   "Create a new group",
		Aliases: []string{"new"},
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			stop := printer.Spinner("Creating group...")
			group, err := client.Groups.Create(context.Background(), models.GroupCreate{
				Name:    name,
				UserIDs: userIDs,
			})
			stop()
			if err != nil {
				return err
			}

			printer.Resource(group, fmt.Sprintf("Created group %q (ID: %d)", group.Name, group.ID))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Group name (required)")
	cmd.Flags().IntSliceVar(&userIDs, "user-ids", nil, "User IDs to add to the group")
	cmd.MarkFlagRequired("name")
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
