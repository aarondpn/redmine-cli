package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
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
		Long:  "Search for wiki pages in Redmine.\n\n" + defaultProjectHelp,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			var notice string
			project, notice, err = resolveSearchProject(context.Background(), f, project, scope)
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Searching wiki pages...")
			result, err := ops.Search(context.Background(), client, ops.SearchInput{
				Query:      query,
				ProjectID:  project,
				Scope:      scope,
				AllWords:   allWords,
				TitlesOnly: titlesOnly,
				WikiPages:  true,
				Limit:      cmdutil.OpsLimit(limit),
				Offset:     offset,
			})
			stop()
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}
			printDefaultProjectNotice(printer, notice)
			results, total := result.Results, result.TotalCount

			if cmdutil.HandleEmpty(printer, results, "wiki pages") {
				return nil
			}

			printResults(printer, results, total, limit, offset)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", projectFlagUsage)
	cmd.Flags().StringVar(&scope, "scope", "", scopeFlagUsage)
	cmd.Flags().BoolVar(&allWords, "all-words", false, "Match all query words")
	cmd.Flags().BoolVar(&titlesOnly, "titles-only", false, "Search titles only")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
