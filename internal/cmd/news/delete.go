package news

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdDelete(f *cmdutil.Factory) *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:     "delete <id>",
		Aliases: []string{"rm"},
		Short:   "Delete a news item",
		Long:    "Permanently delete a news item together with its comments. This action cannot be undone.",
		Example: `  # Delete after confirmation
  redmine news delete 12

  # Skip the confirmation
  redmine news delete 12 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid news ID: %s", args[0])
			}

			printer := f.Printer("")

			if !force {
				msg := fmt.Sprintf("Are you sure you want to delete news %d?", id)
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Outcome(false, output.ActionDeleted, "news", id, "Delete cancelled")
					return nil
				}
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			stop := printer.Spinner("Deleting news...")
			_, err = ops.DeleteNews(cmdutil.Context(cmd), client, ops.DeleteNewsInput{ID: id})
			stop()
			if err != nil {
				return fmt.Errorf("failed to delete news %d: %w", id, err)
			}

			printer.Action(output.ActionDeleted, "news", id, fmt.Sprintf("Deleted news %d", id))
			return nil
		},
	}

	cmdutil.AddForceFlag(cmd, &force)

	return cmd
}
