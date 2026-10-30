package news

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		title       string
		summary     string
		description string
		attach      []string
	)

	cmd := &cobra.Command{
		Use:     "update <id>",
		Aliases: []string{"edit"},
		Short:   "Update a news item",
		Long:    "Update the title, summary or description of a news item. Only the fields you pass are changed; --summary \"\" clears the summary.",
		Example: `  # Change the title
  redmine news update 12 --title "Release 2.0 (final)"

  # Replace the description from a file
  redmine news update 12 --description-file news.textile

  # Clear the summary and attach a file
  redmine news update 12 --summary "" --attach ./notes.pdf`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid news ID: %s", args[0])
			}

			ctx := cmdutil.Context(cmd)

			descGiven, err := cmdutil.ReadTextFileFlag(cmd, f.IOStreams.In, "description", "description-file", &description)
			if err != nil {
				return err
			}
			if err := cmdutil.RejectEmptyTextFile(cmd, "description-file", description, ""); err != nil {
				return err
			}

			input := ops.UpdateNewsInput{ID: id}
			if cmd.Flags().Changed("title") {
				input.Title = &title
			}
			if cmd.Flags().Changed("summary") {
				input.Summary = &summary
			}
			if descGiven {
				input.Description = &description
			}
			if input.Title == nil && input.Summary == nil && input.Description == nil && len(attach) == 0 {
				return errors.New("nothing to change: pass --title, --summary, --description, --description-file or --attach")
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")

			if len(attach) > 0 {
				var uploads []models.Upload
				uploads, err = cmdutil.UploadAttachments(ctx, client, attach)
				if err != nil {
					return err
				}
				input.Uploads = uploads
			}

			stop := printer.Spinner("Updating news...")
			_, err = ops.UpdateNews(ctx, client, input)
			stop()
			if err != nil {
				return fmt.Errorf("failed to update news %d: %w", id, err)
			}

			printer.Action(output.ActionUpdated, "news", id, fmt.Sprintf("Updated news %d", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&title, "title", "", "New title, max 60 characters")
	cmd.Flags().StringVar(&summary, "summary", "", "New summary, max 255 characters (empty clears it)")
	cmd.Flags().StringVarP(&description, "description", "d", "", "New body")
	cmdutil.AddTextFileFlag(cmd, "description", "description-file", "F")
	cmd.Flags().StringArrayVar(&attach, "attach", nil, "Path to file to attach (repeatable)")

	return cmd
}
