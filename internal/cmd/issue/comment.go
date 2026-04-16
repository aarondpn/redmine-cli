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

// NewCmdComment creates the issues comment command.
func NewCmdComment(f *cmdutil.Factory) *cobra.Command {
	var message string

	cmd := &cobra.Command{
		Use:   "comment <id>",
		Short: "Add a comment to an issue",
		Long:  "Add a comment (note) to an existing issue.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}

			if message == "" {
				return fmt.Errorf("--message is required")
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			update := models.IssueUpdate{
				Notes: &message,
			}

			printer := f.Printer("")
			stop := printer.Spinner("Adding comment...")
			err = client.Issues.Update(context.Background(), id, update)
			stop()
			if err != nil {
				return fmt.Errorf("failed to add comment to issue %s: %w", fmt.Sprintf("#%d", id), err)
			}

			printer.Action(output.ActionCommented, "issue", id, fmt.Sprintf("Added comment to issue #%d", id))
			return nil
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Comment message")

	return cmd
}
