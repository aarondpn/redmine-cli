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

func newCmdMembershipDelete(f *cmdutil.Factory) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"rm"},
		Short:   "Remove a membership",
		Long:    "Delete a membership, removing a user or group from a project.",
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

			printer := f.Printer("")

			if !force {
				msg := fmt.Sprintf("Are you sure you want to delete membership %d?", id)
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Outcome(false, output.ActionDeleted, "membership", id, "Delete cancelled")
					return nil
				}
			}

			stop := printer.Spinner("Deleting membership...")
			_, err = ops.DeleteMembership(context.Background(), client, ops.DeleteMembershipInput{ID: id})
			stop()
			if err != nil {
				return err
			}

			printer.Action(output.ActionDeleted, "membership", id, fmt.Sprintf("Deleted membership %d", id))
			return nil
		},
	}

	cmdutil.AddForceFlag(cmd, &force)
	return cmd
}
