package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/api"
	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
)

func newCmdSearchIssues(f *cmdutil.Factory) *cobra.Command {
	var (
		project     string
		scope       string
		allWords    bool
		titlesOnly  bool
		openIssues  bool
		attachments string
		limit       int
		offset      int
		format      string
	)

	cmd := &cobra.Command{
		Use:   "issues <query>",
		Short: "Search issues",
		Long:  "Search for issues in Redmine.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			project, err = cmdutil.DefaultProjectIdentifier(context.Background(), f, project)
			if err != nil {
				return err
			}

			params := api.SearchParams{
				Query:       query,
				ProjectID:   project,
				Scope:       scope,
				AllWords:    allWords,
				TitlesOnly:  titlesOnly,
				OpenIssues:  openIssues,
				Attachments: attachments,
				Issues:      true,
				Limit:       limit,
				Offset:      offset,
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Searching issues...")
			results, total, err := client.Search.Search(context.Background(), params)
			stop()
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			if cmdutil.HandleEmpty(printer, results, "issues") {
				return nil
			}

			switch printer.Format() {
			case output.FormatJSON:
				printer.JSON(results)
			case output.FormatCSV:
				headers := []string{"ID", "Title", "Date"}
				rows := make([][]string, len(results))
				for i, r := range results {
					rows[i] = []string{
						fmt.Sprintf("%d", r.ID),
						r.Title,
						r.DateTime,
					}
				}
				printer.CSV(headers, rows)
			default:
				headers := []string{"ID", "Title", "Date"}
				rows := make([][]string, len(results))
				for i, r := range results {
					rows[i] = []string{
						output.StyleID.Render(fmt.Sprintf("#%d", r.ID)),
						r.Title,
						formatDate(r.DateTime),
					}
				}
				printer.Table(headers, rows)
			}

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(results), Total: total, Limit: limit, Offset: offset, Noun: "results",
			})

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Limit search to a project name, identifier, or ID")
	cmd.Flags().StringVar(&scope, "scope", "", "Search scope: all, my_projects, subprojects")
	cmd.Flags().BoolVar(&allWords, "all-words", false, "Match all query words")
	cmd.Flags().BoolVar(&titlesOnly, "titles-only", false, "Search titles only")
	cmd.Flags().BoolVar(&openIssues, "open-issues", false, "Only return open issues")
	cmd.Flags().StringVar(&attachments, "attachments", "", "Attachment search: 0, 1, or only")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
