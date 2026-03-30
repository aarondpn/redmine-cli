package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/api"
	"github.com/aarondpn/redmine-cli/internal/cmdutil"
)

func newCmdSearchMessages(f *cmdutil.Factory) *cobra.Command {
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
		Use:   "messages <query>",
		Short: "Search forum messages",
		Long:  "Search for forum messages in Redmine.",
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
				Query:      query,
				ProjectID:  project,
				Scope:      scope,
				AllWords:   allWords,
				TitlesOnly: titlesOnly,
				Messages:   true,
				Limit:      limit,
				Offset:     offset,
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Searching messages...")
			results, total, err := client.Search.Search(context.Background(), params)
			stop()
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			if cmdutil.HandleEmpty(printer, results, "messages") {
				return nil
			}

			printResults(printer, results, total, limit, offset)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Limit search to a project name, identifier, or ID")
	cmd.Flags().StringVar(&scope, "scope", "", "Search scope: all, my_projects, subprojects")
	cmd.Flags().BoolVar(&allWords, "all-words", false, "Match all query words")
	cmd.Flags().BoolVar(&titlesOnly, "titles-only", false, "Search titles only")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
