package attachment

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdGet(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "get <id>",
		Aliases: []string{"show", "view", "metadata"},
		Short:   "Show attachment metadata",
		Long: "Display metadata for a single attachment: filename, size, content type, " +
			"description, author, creation time, and download URL.\n\n" +
			"Use `redmine attachments download <id>` to fetch the file itself.",
		Example: `  # Show metadata in a table
  redmine attachments get 42

  # JSON output (for scripting)
  redmine attachments get 42 -o json

  # CSV output
  redmine attachments get 42 -o csv`,
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
			printer := f.Printer(format)

			stop := printer.Spinner("Fetching attachment...")
			att, err := ops.GetAttachment(context.Background(), client, ops.GetAttachmentInput{ID: id})
			stop()
			if err != nil {
				return fmt.Errorf("failed to get attachment %d: %w", id, err)
			}

			switch printer.Format() {
			case output.FormatJSON:
				printer.JSON(att)
			case output.FormatCSV:
				printer.CSV(attachmentHeaders, [][]string{attachmentRow(att)})
			default:
				renderAttachmentDetail(printer, att)
			}
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}

var attachmentHeaders = []string{"ID", "Filename", "Size", "Content-Type", "Author", "Created", "Description"}

func attachmentRow(att *models.Attachment) []string {
	return []string{
		strconv.Itoa(att.ID),
		att.Filename,
		strconv.FormatInt(att.Filesize, 10),
		att.ContentType,
		att.Author.Name,
		att.CreatedOn,
		att.Description,
	}
}

func renderAttachmentDetail(printer output.Printer, att *models.Attachment) {
	pairs := []output.KeyValue{
		{Key: "ID", Value: output.StyleID.Render(strconv.Itoa(att.ID))},
		{Key: "Filename", Value: att.Filename},
		{Key: "Size", Value: fmt.Sprintf("%d bytes", att.Filesize)},
		{Key: "Content-Type", Value: att.ContentType},
		{Key: "Author", Value: att.Author.Name},
		{Key: "Created", Value: att.CreatedOn},
	}
	if att.Description != "" {
		pairs = append(pairs, output.KeyValue{Key: "Description", Value: att.Description})
	}
	if att.ContentURL != "" {
		pairs = append(pairs, output.KeyValue{Key: "Content URL", Value: att.ContentURL})
	}
	printer.Detail(pairs)
}
