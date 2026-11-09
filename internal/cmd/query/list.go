package query

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdQueryList(f *cmdutil.Factory) *cobra.Command {
	var (
		queryType string
		format    string
		limit     int
		offset    int
	)

	cmd := &cobra.Command{
		Use:     "list",
		Args:    cobra.NoArgs,
		Aliases: []string{"ls"},
		Short:   "List saved queries",
		Long: `List saved queries (custom filters) visible to the authenticated user. The list includes both global queries and queries scoped to a project.

Issue queries run with "issues list --query"; time entry queries (--type time_entry) run with "time list --query".`,
		Example: `  # All saved queries you can see
  redmine queries list

  # Saved time entry queries, for time list --query / --query-id
  redmine queries list --type time_entry

  # JSON output for piping into jq or another tool
  redmine queries list -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if _, err := ops.QueryClass(queryType); err != nil {
				return err
			}
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			stop := printer.Spinner("Fetching saved queries...")
			result, err := ops.ListQueries(cmdutil.Context(cmd), client, ops.ListQueriesInput{
				Type:   queryType,
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

			cmdutil.RenderCollection(printer, queries, []string{"ID", "Name", "Visibility", "Scope"}, func(q models.SavedQuery) []string {
				id := output.StyleID.Render(fmt.Sprintf("%d", q.ID))
				return []string{id, q.Name, queryVisibility(q), queryScope(q)}
			})

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(queries), Total: result.TotalCount, Limit: limit, Offset: offset, Noun: "queries",
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&queryType, "type", ops.QueryTypeIssue, "Kind of saved query: issue or time_entry")
	_ = cmd.RegisterFlagCompletionFunc("type", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		return []string{ops.QueryTypeIssue, ops.QueryTypeTimeEntry}, cobra.ShellCompDirectiveNoFileComp
	})
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
