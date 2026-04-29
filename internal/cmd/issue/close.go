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
		Long:  "Close an issue by setting its status to the first closed status.",
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
			stop := printer.Spinner("Closing issue...")
			_, err = ops.CloseIssue(context.Background(), client, ops.CloseIssueInput{
				ID:    id,
				Notes: note,
			})
			stop()
			if err != nil {
				return fmt.Errorf("failed to close issue %s: %w", fmt.Sprintf("#%d", id), err)
			}

			printer.Action(output.ActionClosed, "issue", id, fmt.Sprintf("Closed issue #%d", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&note, "note", "", "Add a note when closing")

	return cmd
}
