package issue

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/output"
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
			stop := printer.Spinner("Fetching statuses...")
			statuses, err := client.Statuses.List(context.Background())
			stop()
			if err != nil {
				return fmt.Errorf("failed to fetch statuses: %w", err)
			}

			var openStatusID int
			for _, s := range statuses {
				if !s.IsClosed {
					openStatusID = s.ID
					break
				}
			}
			if openStatusID == 0 {
				return fmt.Errorf("no open status found")
			}

			update := models.IssueUpdate{
				StatusID: &openStatusID,
			}
			if note != "" {
				update.Notes = &note
			}

			stop = printer.Spinner("Reopening issue...")
			err = client.Issues.Update(context.Background(), id, update)
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
