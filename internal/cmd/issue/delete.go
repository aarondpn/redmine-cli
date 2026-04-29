package issue

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// NewCmdDelete creates the issues delete command.
func NewCmdDelete(f *cmdutil.Factory) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"rm"},
		Short:   "Delete an issue",
		Long:    "Permanently delete an issue. This action cannot be undone.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}

			printer := f.Printer("")

			if !force {
				msg := fmt.Sprintf("Are you sure you want to delete issue #%d?", id)
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Outcome(false, output.ActionDeleted, "issue", id, "Delete cancelled")
					return nil
				}
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			stop := printer.Spinner("Deleting issue...")
			_, err = ops.DeleteIssue(context.Background(), client, ops.DeleteIssueInput{ID: id})
			stop()
			if err != nil {
				return fmt.Errorf("failed to delete issue #%d: %w", id, err)
			}

			printer.Action(output.ActionDeleted, "issue", id, fmt.Sprintf("Deleted issue #%d", id))
			return nil
		},
	}

	cmdutil.AddForceFlag(cmd, &force)

	return cmd
}
