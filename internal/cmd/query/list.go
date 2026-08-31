package query

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdQueryList(f *cmdutil.Factory) *cobra.Command {
	var (
		format string
		limit  int
		offset int
	)

	cmd := &cobra.Command{
		Use:     "list",
		Args:    cobra.NoArgs,
		Aliases: []string{"ls"},
		Short:   "List saved queries",
		Long:    "List saved queries (custom filters) visible to the authenticated user. The list includes both global queries and queries scoped to a project.",
		Example: `  # All saved queries you can see
  redmine queries list

  # JSON output for piping into jq or another tool
  redmine queries list -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			stop := printer.Spinner("Fetching saved queries...")
			result, err := ops.ListQueries(context.Background(), client, ops.ListQueriesInput{
				Limit:  cmdutil.OpsLimit(limit),
				Offset: offset,
			})
			stop()
			if err != nil {
				return err
			}
			queries := result.Queries

			if cmdutil.HandleEmpty(printer, queries, "queries") {
				return nil
			}

			cmdutil.RenderCollection(printer, queries, []string{"ID", "Name", "Visibility", "Scope"}, func(q models.SavedQuery, styled bool) []string {
				id := fmt.Sprintf("%d", q.ID)
				if styled {
					id = output.StyleID.Render(id)
				}
				return []string{id, q.Name, queryVisibility(q), queryScope(q)}
			})

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(queries), Total: result.TotalCount, Limit: limit, Offset: offset, Noun: "queries",
			})
			return nil
		},
	}

	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}

func queryVisibility(q models.SavedQuery) string {
	if q.IsPublic {
		return "public"
	}
	return "private"
}

func queryScope(q models.SavedQuery) string {
	if q.ProjectID == nil {
		return "global"
	}
	return fmt.Sprintf("project %d", *q.ProjectID)
}
