package group

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/spf13/cobra"
)

func newCmdGroupDelete(f *cmdutil.Factory) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete <id>",
		Short:   "Delete a group",
		Aliases: []string{"rm"},
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid group ID: %s", args[0])
			}

			printer := f.Printer("")

			if !force {
				msg := fmt.Sprintf("Are you sure you want to delete group %d?", id)
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Warning("Delete cancelled")
					return nil
				}
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			stop := printer.Spinner("Deleting group...")
			err = client.Groups.Delete(context.Background(), id)
			stop()
			if err != nil {
				return err
			}

			printer.Success(fmt.Sprintf("Deleted group %d", id))
			return nil
		},
	}

	cmdutil.AddForceFlag(cmd, &force)
	return cmd
}
