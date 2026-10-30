package attachment

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		filename    string
		description string
		contentType string
	)

	cmd := &cobra.Command{
		Use:     "update <id>",
		Aliases: []string{"edit"},
		Short:   "Rename an attachment or change its description",
		Long: "Change an attachment's filename, description or content type. Only the flags you pass are\n" +
			"changed; an empty --description clears the description. Requires permission to edit the\n" +
			"object the attachment belongs to.",
		Example: `  # Rename an attachment
  redmine attachments update 42 --filename report-final.pdf

  # Set a description
  redmine attachments update 42 --description "Final version"

  # Correct the content type
  redmine attachments update 42 --content-type application/pdf`,
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

			input := ops.UpdateAttachmentInput{ID: id}
			if cmd.Flags().Changed("filename") {
				input.Filename = &filename
			}
			if cmd.Flags().Changed("description") {
				input.Description = &description
			}
			if cmd.Flags().Changed("content-type") {
				input.ContentType = &contentType
			}

			printer := f.Printer("")
			stop := printer.Spinner("Updating attachment...")
			_, err = ops.UpdateAttachment(cmdutil.Context(cmd), client, input)
			stop()
			if err != nil {
				return fmt.Errorf("failed to update attachment %d: %w", id, err)
			}

			printer.Action(output.ActionUpdated, "attachment", id, fmt.Sprintf("Attachment %d updated", id))
			return nil
		},
	}

	cmd.Flags().StringVar(&filename, "filename", "", "New filename")
	cmd.Flags().StringVar(&description, "description", "", "New description (empty clears it)")
	cmd.Flags().StringVar(&contentType, "content-type", "", "New content type, e.g. image/png")
	cmd.MarkFlagsOneRequired("filename", "description", "content-type")

	return cmd
}
