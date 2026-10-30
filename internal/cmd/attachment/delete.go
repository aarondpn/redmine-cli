package attachment

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
		Short:   "Delete an attachment",
		Long: "Permanently delete an attachment. Requires permission to edit the object the attachment\n" +
			"belongs to; deleting an issue attachment is recorded in the issue history.",
		Example: `  # Delete an attachment (asks for confirmation)
  redmine attachments delete 42

  # Skip the confirmation prompt
  redmine attachments delete 42 --force`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid attachment ID: %s", args[0])
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			printer := f.Printer("")
			if !force {
				msg := fmt.Sprintf("Are you sure you want to delete attachment %d?", id)
				if !cmdutil.ConfirmAction(f.IOStreams.In, f.IOStreams.ErrOut, msg) {
					printer.Outcome(false, output.ActionDeleted, "attachment", id, "Delete cancelled")
					return nil
				}
			}

			stop := printer.Spinner("Deleting attachment...")
			_, err = ops.DeleteAttachment(cmdutil.Context(cmd), client, ops.DeleteAttachmentInput{ID: id})
			stop()
			if err != nil {
				return fmt.Errorf("failed to delete attachment %d: %w", id, err)
			}

			printer.Action(output.ActionDeleted, "attachment", id, fmt.Sprintf("Deleted attachment %d", id))
			return nil
		},
	}

	cmdutil.AddForceFlag(cmd, &force)

	return cmd
}
