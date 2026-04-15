package wiki

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
)

func newCmdUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		project  string
		text     string
		title    string
		comments string
		attach   []string
	)

	cmd := &cobra.Command{
		Use:     "update <page>",
		Aliases: []string{"edit"},
		Short:   "Update a wiki page",
		Long:    "Update an existing Redmine wiki page.",
		Example: `  # Update page content
  redmine wiki update MyPage --project myproject --text "Updated content"

  # Update with a change comment (text is preserved when omitted)
  redmine wiki update MyPage --project myproject --comments "Fixed typo"

  # Rename a page
  redmine wiki update MyPage --project myproject --title "New Title"

  # Attach a file
  redmine wiki update MyPage --project myproject --attach ./screenshot.png`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")

			projectID, err := cmdutil.RequireProjectIdentifier(ctx, f, project)
			if err != nil {
				return err
			}

			update := models.WikiPageUpdate{}

			if cmd.Flags().Changed("text") {
				update.Text = &text
			} else {
				// Redmine requires the text field on every PUT.
				// Fetch the current page and resend its text unchanged.
				current, err := client.Wikis.Get(ctx, projectID, args[0], nil)
				if err != nil {
					return fmt.Errorf("failed to fetch current wiki page %q: %w", args[0], err)
				}
				update.Text = &current.Text
			}
			if cmd.Flags().Changed("title") {
				update.Title = &title
			}
			if cmd.Flags().Changed("comments") {
				update.Comments = &comments
			}

			if len(attach) > 0 {
				uploads, err := cmdutil.UploadAttachments(ctx, client, attach)
				if err != nil {
					return err
				}
				update.Uploads = uploads
			}

			stop := printer.Spinner("Updating wiki page...")
			err = client.Wikis.Update(ctx, projectID, args[0], update)
			stop()
			if err != nil {
				return fmt.Errorf("failed to update wiki page %q: %w", args[0], err)
			}

			printer.Success(fmt.Sprintf("Wiki page %q updated", args[0]))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier or ID (required if no default)")
	cmd.Flags().StringVarP(&text, "text", "t", "", "Page content in Textile/Markdown")
	cmd.Flags().StringVar(&title, "title", "", "Display title")
	cmd.Flags().StringVar(&comments, "comments", "", "Change comment")
	cmd.Flags().StringArrayVar(&attach, "attach", nil, "Path to file to attach (repeatable)")

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
