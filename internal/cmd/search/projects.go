package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
)

func newCmdSearchProjects(f *cmdutil.Factory) *cobra.Command {
	var (
		scope      string
		allWords   bool
		titlesOnly bool
		limit      int
		offset     int
		format     string
	)

	cmd := &cobra.Command{
		Use:   "projects <query>",
		Short: "Search projects",
		Long:  "Search for projects in Redmine.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			params := api.SearchParams{
				Query:      query,
				Scope:      scope,
				AllWords:   allWords,
				TitlesOnly: titlesOnly,
				Projects:   true,
				Limit:      limit,
				Offset:     offset,
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Searching projects...")
			results, total, err := client.Search.Search(context.Background(), params)
			stop()
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			if cmdutil.HandleEmpty(printer, results, "projects") {
				return nil
			}

			printResults(printer, results, total, limit, offset)
			return nil
		},
	}

	cmd.Flags().StringVar(&scope, "scope", "", "Search scope: all, my_projects, subprojects")
	cmd.Flags().BoolVar(&allWords, "all-words", false, "Match all query words")
	cmd.Flags().BoolVar(&titlesOnly, "titles-only", false, "Search titles only")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}
