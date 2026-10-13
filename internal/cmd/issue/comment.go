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

// NewCmdComment creates the issues comment command.
func NewCmdComment(f *cmdutil.Factory) *cobra.Command {
	var message string

	cmd := &cobra.Command{
		Use:   "comment <id>",
		Short: "Add a comment to an issue",
		Long:  "Add a comment (note) to an existing issue.",
		Example: `  # Add a short comment
  redmine issues comment 123 --message "Deployed to staging"

  # Read a long Textile/Markdown comment from a file
  redmine issues comment 123 --message-file notes.textile

  # Read the comment from standard input
  git log -1 --format=%B | redmine issues comment 123 -F -`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue ID: %s", args[0])
			}

			if _, err := cmdutil.ReadTextFileFlag(cmd, f.IOStreams.In, "message", "message-file", &message); err != nil {
				return err
			}
			if message == "" {
				if cmd.Flags().Changed("message-file") {
					return fmt.Errorf("--message-file is empty")
				}
				return fmt.Errorf("--message or --message-file is required")
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			printer := f.Printer("")
			stop := printer.Spinner("Adding comment...")
			_, err = ops.AddIssueComment(context.Background(), client, ops.AddIssueCommentInput{
				ID:    id,
				Notes: message,
			})
			stop()
			if err != nil {
				return fmt.Errorf("failed to add comment to issue %s: %w", fmt.Sprintf("#%d", id), err)
			}

			printer.Action(output.ActionCommented, "issue", id, fmt.Sprintf("Added comment to issue #%d", id))
			return nil
		},
	}

	cmd.Flags().StringVarP(&message, "message", "m", "", "Comment message")
	cmdutil.AddTextFileFlag(cmd, "message", "message-file", "F")

	return cmd
}
