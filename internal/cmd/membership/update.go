package membership

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdMembershipUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		roleIDs []int
		roles   []string
	)

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

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			resolvedRoleIDs, err := resolveRoleIDs(
				context.Background(),
				client,
				roleIDs,
				roles,
				cmd.Flags().Changed("role-ids"),
				cmd.Flags().Changed("roles"),
			)
			if err != nil {
				return err
			}

			printer := f.Printer("")

			stop := printer.Spinner("Updating membership...")
			_, err = ops.UpdateMembership(context.Background(), client, ops.UpdateMembershipInput{
				ID:      id,
				RoleIDs: resolvedRoleIDs,
			})
			stop()
			if err != nil {
				return err
			}

			printer.Action(output.ActionUpdated, "membership", id, fmt.Sprintf("Updated membership %d", id))
			return nil
		},
	}

	cmd.Flags().IntSliceVar(&roleIDs, "role-ids", nil, "Role IDs to assign")
	cmd.Flags().StringSliceVar(&roles, "roles", nil, "Role names or IDs to assign (repeatable or comma-separated)")
	cmd.MarkFlagsMutuallyExclusive("role-ids", "roles")
	_ = cmd.RegisterFlagCompletionFunc("roles", cmdutil.CompleteRoles(f))
	return cmd
}
