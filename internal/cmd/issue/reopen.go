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
		Long:  "Reopen a closed issue by setting its status to the first non-closed status.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			printer := f.Printer("")
			stop := printer.Spinner("Reopening issue...")
			_, err = ops.ReopenIssue(context.Background(), client, ops.ReopenIssueInput{
				ID:    id,
				Notes: note,
			})
			stop()
			if err != nil {
				return fmt.Errorf("failed to reopen issue %s: %w", fmt.Sprintf("#%d", id), err)
			}

			printer.Action(output.ActionReopened, "issue", id, fmt.Sprintf("Reopened issue #%d", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&note, "note", "", "Add a note when reopening")

	return cmd
}
