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

// NewCmdClose creates the issues close command.
func NewCmdClose(f *cmdutil.Factory) *cobra.Command {
	var note string

	cmd := &cobra.Command{
		Use:   "close <id>",
		Short: "Close an issue",
		Long: "Close an issue by setting its status to the first closed status.\n\n" +
			"The issue is read back afterwards. If Redmine silently ignored the " +
			"transition (its workflow does not allow it for your role and the issue's " +
			"tracker, or the issue has open subtasks or open blocking issues), the " +
			"command warns on stderr, reports ok:false in JSON output, lists the allowed " +
			"statuses (Redmine 5.0+), and exits with status 1.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}

			if _, err := cmdutil.ReadTextFileFlag(cmd, f.IOStreams.In, "note", "note-file", &note); err != nil {
				return err
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			printer := f.Printer("")
			stop := printer.Spinner("Closing issue...")
			result, err := ops.CloseIssue(context.Background(), client, ops.CloseIssueInput{
				ID:    id,
				Notes: note,
			})
			stop()
			if err != nil {
				if silent := reportStatusNotApplied(printer, output.ActionClosed, id, err); silent != nil {
					return silent
				}
				return fmt.Errorf("failed to close issue %s: %w", fmt.Sprintf("#%d", id), err)
			}

			printWarnings(printer, result.Warnings)
			printer.Action(output.ActionClosed, "issue", id, fmt.Sprintf("Closed issue #%d", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&note, "note", "", "Add a note when closing")
	cmdutil.AddTextFileFlag(cmd, "note", "note-file", "")

	return cmd
}
