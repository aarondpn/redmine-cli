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

// NewCmdReopen creates the issues reopen command.
func NewCmdReopen(f *cmdutil.Factory) *cobra.Command {
	var note string

	cmd := &cobra.Command{
		Use:   "reopen <id>",
		Short: "Reopen a closed issue",
		Long: "Reopen a closed issue by setting its status to the first non-closed status.\n\n" +
			"The issue is read back afterwards. An issue that is already open (in any " +
			"open status) counts as reopened. If Redmine silently ignored the " +
			"transition (its workflow does not allow it for your role and the issue's " +
			"tracker), the command fails with error code status_not_applied, listing " +
			"the allowed statuses (Redmine 5.0+).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}

			if _, err := cmdutil.ReadTextFileFlag(cmd, f.IOStreams.In, "note", "note-file", &note); err != nil {
				return err
			}
			if err := cmdutil.RejectEmptyTextFile(cmd, "note-file", note, ""); err != nil {
				return err
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			printer := f.Printer("")
			stop := printer.Spinner("Reopening issue...")
			result, err := ops.ReopenIssue(context.Background(), client, ops.ReopenIssueInput{
				ID:    id,
				Notes: note,
			})
			stop()
			if err != nil {
				if coded := statusNotAppliedError(err); coded != nil {
					return coded
				}
				return fmt.Errorf("failed to reopen issue %s: %w", fmt.Sprintf("#%d", id), err)
			}

			printWarnings(printer, result.Warnings)
			printer.Action(output.ActionReopened, "issue", id, fmt.Sprintf("Reopened issue #%d", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&note, "note", "", "Add a note when reopening")
	cmdutil.AddTextFileFlag(cmd, "note", "note-file", "")

	return cmd
}
