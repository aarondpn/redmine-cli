package wiki

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
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
		Aliases: []string{"ls"},
		Short:   "List wiki pages",
		Long:    "List wiki pages in a Redmine project.",
		Example: `  # List wiki pages in a project
  redmine wiki list --project myproject

  # JSON output
  redmine wiki list --project myproject --output json

  # Paginate results
  redmine wiki list --project myproject --limit 10 --offset 20`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			projectID, err := cmdutil.RequireProjectIdentifier(ctx, f, project)
			if err != nil {
				return err
			}

			stop := printer.Spinner("Fetching wiki pages...")
			pages, total, err := client.Wikis.List(ctx, projectID, limit, offset)
			stop()
			if err != nil {
				return fmt.Errorf("failed to list wiki pages: %w", err)
			}

			if cmdutil.HandleEmpty(printer, pages, "wiki pages") {
				return nil
			}

			cmdutil.RenderCollection(printer, pages, []string{"Title", "Updated"}, func(p models.WikiPageIndex, styled bool) []string {
				title := p.Title
				if styled {
					title = output.StyleID.Render(title)
				}
				return []string{title, p.UpdatedOn}
			})

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(pages), Total: total, Limit: limit, Offset: offset, Noun: "wiki pages",
			})

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier or ID (required if no default)")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
