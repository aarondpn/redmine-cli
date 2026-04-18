package membership

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdMembershipUpdate(f *cmdutil.Factory) *cobra.Command {
	var roleIDs []int

	cmd := &cobra.Command{
		Use:     "update <id>",
		Aliases: []string{"edit"},
		Short:   "Update membership roles",
		Long:    "Update the roles assigned to a membership.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("membership ID must be a number: %s", args[0])
			}

			if !cmd.Flags().Changed("role-ids") {
				return fmt.Errorf("--role-ids is required")
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			printer := f.Printer("")

			stop := printer.Spinner("Updating membership...")
			err = client.Memberships.Update(context.Background(), id, models.MembershipUpdate{
				RoleIDs: roleIDs,
			})
			stop()
			if err != nil {
				return err
			}

			printer.Action(output.ActionUpdated, "membership", id, fmt.Sprintf("Updated membership %d", id))
			return nil
		},
	}

	cmd.Flags().IntSliceVar(&roleIDs, "role-ids", nil, "Role IDs to assign (required)")
	return cmd
}
