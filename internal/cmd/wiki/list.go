package wiki

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
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
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			projectID, err := cmdutil.RequireProjectIdentifier(context.Background(), f, project)
			if err != nil {
				return err
			}

			pages, total, err := client.Wikis.List(context.Background(), projectID, limit, offset)
			if err != nil {
				return err
			}

			if cmdutil.HandleEmpty(printer, pages, "wiki pages") {
				return nil
			}

			headers := []string{"Title", "Updated"}

			switch printer.Format() {
			case output.FormatJSON:
				printer.JSON(pages)
			case output.FormatCSV:
				rows := make([][]string, 0, len(pages))
				for _, p := range pages {
					rows = append(rows, []string{p.Title, p.UpdatedOn})
				}
				printer.CSV(headers, rows)
			default:
				rows := make([][]string, 0, len(pages))
				for _, p := range pages {
					rows = append(rows, []string{
						output.StyleID.Render(p.Title),
						p.UpdatedOn,
					})
				}
				printer.Table(headers, rows)
			}

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(pages), Total: total, Limit: limit, Offset: offset, Noun: "wiki pages",
			})

			return nil
		},
	}

	cmd.Flags().StringVarP(&project, "project", "p", "", "Project identifier or ID (required if no default)")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
