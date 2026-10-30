package news

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdList(f *cmdutil.Factory) *cobra.Command {
	var (
		project string
		limit   int
		offset  int
		format  string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Args:    cobra.NoArgs,
		Aliases: []string{"ls"},
		Short:   "List news",
		Long:    "List news, newest first. Without --project (and without a default project) the news of all visible projects is listed.",
		Example: `  # News of one project
  redmine news list --project myproject

  # News across all visible projects
  redmine news list

  # Page through the news
  redmine news list --project myproject --limit 10 --offset 10

  # Titles only
  redmine news list --project myproject --jq '.[].title'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdutil.Context(cmd)
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			projectID, err := cmdutil.DefaultProjectIdentifier(ctx, f, project)
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching news...")
			result, err := ops.ListNews(ctx, client, ops.ListNewsInput{
				ProjectID: projectID,
				Limit:     cmdutil.OpsLimit(limit),
				Offset:    offset,
			})
			stop()
			if err != nil {
				return fmt.Errorf("failed to list news: %w", err)
			}

			if cmdutil.HandleEmpty(printer, result.News, "news") {
				return nil
			}

			cmdutil.RenderCollection(printer, result.News,
				[]string{"ID", "Project", "Title", "Author", "Created"},
				func(n models.News) []string {
					return []string{
						output.StyleID.Render(fmt.Sprintf("%d", n.ID)),
						n.Project.Name,
						n.Title,
						n.Author.Name,
						n.CreatedOn,
					}
				})

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(result.News), Total: result.TotalCount, Limit: limit, Offset: offset, Noun: "news",
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID (falls back to the default project; omit for all projects)")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
