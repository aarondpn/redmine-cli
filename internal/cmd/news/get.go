package news

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdGet(f *cmdutil.Factory) *cobra.Command {
	var (
		include []string
		format  string
	)

	cmd := &cobra.Command{
		Use:     "get <id>",
		Aliases: []string{"show", "view"},
		Short:   "Get news details",
		Long: "Display a news item. Use --include to add its attachments, comments and " +
			"reactions (reactions need Redmine 7.1+). News comments are read-only over the API.",
		Example: `  # View a news item
  redmine news get 12

  # Include attachments and comments
  redmine news get 12 --include attachments,comments

  # JSON output
  redmine news get 12 --include comments -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil || id <= 0 {
				return fmt.Errorf("invalid news ID: %s", args[0])
			}
			if err := validateGetIncludes(include); err != nil {
				return err
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching news...")
			item, err := ops.GetNews(cmdutil.Context(cmd), client, ops.GetNewsInput{
				ID:       id,
				Includes: cmdutil.MergeIncludes(include),
			})
			stop()
			if err != nil {
				return fmt.Errorf("failed to get news %d: %w", id, err)
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(item)
				return nil
			}

			printer.Detail([]output.KeyValue{
				{Key: "ID", Value: output.StyleID.Render(strconv.Itoa(item.ID))},
				{Key: "Project", Value: item.Project.Name},
				{Key: "Title", Value: item.Title},
				{Key: "Summary", Value: item.Summary},
				{Key: "Author", Value: item.Author.Name},
				{Key: "Created", Value: item.CreatedOn},
				{Key: "Description", Value: item.Description},
			})

			if len(item.Comments) > 0 {
				rows := make([][]string, len(item.Comments))
				for i, c := range item.Comments {
					rows[i] = []string{output.StyleID.Render(strconv.Itoa(c.ID)), c.Author.Name, c.Content}
				}
				printer.Table([]string{"ID", "Author", "Comment"}, rows)
			}
			if len(item.Attachments) > 0 {
				rows := make([][]string, len(item.Attachments))
				for i, a := range item.Attachments {
					rows[i] = []string{
						output.StyleID.Render(strconv.Itoa(a.ID)),
						a.Filename,
						strconv.FormatInt(a.Filesize, 10),
						a.ContentType,
					}
				}
				printer.Table([]string{"ID", "Filename", "Size", "Content-Type"}, rows)
			}
			return nil
		},
	}

	cmd.Flags().StringSliceVar(&include, "include", nil, "Include related data: attachments, comments, reactions (7.1+) (repeatable or comma-separated)")
	_ = cmd.RegisterFlagCompletionFunc("include", completeGetIncludes)
	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}
