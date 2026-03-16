package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/api"
	"github.com/aarondpn/redmine-cli/internal/cmdutil"
)

func newCmdSearchWiki(f *cmdutil.Factory) *cobra.Command {
	var (
		project    string
		scope      string
		allWords   bool
		titlesOnly bool
		limit      int
		offset     int
		format     string
	)

	cmd := &cobra.Command{
		Use:   "wiki <query>",
		Short: "Search wiki pages",
		Long:  "Search for wiki pages in Redmine.",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			if project == "" {
				cfg, err := f.Config()
				if err == nil && cfg.DefaultProject != "" {
					project = cfg.DefaultProject
				}
			}

			params := api.SearchParams{
				Query:      query,
				ProjectID:  project,
				Scope:      scope,
				AllWords:   allWords,
				TitlesOnly: titlesOnly,
				WikiPages:  true,
				Limit:      limit,
				Offset:     offset,
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Searching wiki pages...")
			results, total, err := client.Search.Search(context.Background(), params)
			stop()
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			if len(results) == 0 {
				printer.Warning("No wiki pages found")
				return nil
			}

			printResults(printer, results, total, limit, offset)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Limit search to a project identifier")
	cmd.Flags().StringVar(&scope, "scope", "", "Search scope: all, my_projects, subprojects")
	cmd.Flags().BoolVar(&allWords, "all-words", false, "Match all query words")
	cmd.Flags().BoolVar(&titlesOnly, "titles-only", false, "Search titles only")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}
