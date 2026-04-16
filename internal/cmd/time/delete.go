package time

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
)

func newCmdTimeDelete(f *cmdutil.Factory) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"rm"},
		Short:   "Delete a time entry",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid time entry ID: %s", args[0])
			}

			printer := f.Printer("")

			if !force {
				msg := fmt.Sprintf("Are you sure you want to delete time entry #%d?", id)
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Outcome(false, output.ActionDeleted, "time_entry", id, "Delete cancelled")
					return nil
				}
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			if err := client.TimeEntries.Delete(context.Background(), id); err != nil {
				return err
			}

			printer.Action(output.ActionDeleted, "time_entry", id, fmt.Sprintf("Time entry #%d deleted", id))

			return nil
		},
	}

	cmdutil.AddForceFlag(cmd, &force)

	return cmd
}
