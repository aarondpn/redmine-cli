package search

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
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
		Long:  "Search for issues in Redmine.\n\n" + defaultProjectHelp,
		Example: `  # Search open issues across all projects
  redmine search issues "login timeout" --scope all --open-issues

  # Match titles only within one project
  redmine search issues "release checklist" --project myproject --titles-only`,
		Args: cobra.MinimumNArgs(1),
		// Stays on direct client.Search.Search: --attachments has no field on
		// ops.SearchInput, and migrating would silently drop user-visible
		// functionality. Revisit when the ops layer exposes an Attachments knob.
		RunE: func(cmd *cobra.Command, args []string) error {
			query := strings.Join(args, " ")
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			var notice string
			project, notice, err = resolveSearchProject(cmdutil.Context(cmd), f, project, scope)
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
			results, total, err := client.Search.Search(cmdutil.Context(cmd), params)
			stop()
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}
			printDefaultProjectNotice(printer, notice)

			if cmdutil.HandleEmpty(printer, results, "issues") {
				return nil
			}

			cmdutil.RenderCollection(printer, results, []string{"ID", "Title", "Date"}, func(r models.SearchResult, styled bool) []string {
				id := fmt.Sprintf("%d", r.ID)
				date := r.DateTime
				if styled {
					id = output.StyleID.Render(fmt.Sprintf("#%d", r.ID))
					date = formatDate(r.DateTime)
				}
				return []string{id, r.Title, date}
			})

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(results), Total: total, Limit: limit, Offset: offset, Noun: "results",
			})

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", projectFlagUsage)
	cmd.Flags().StringVar(&scope, "scope", "", scopeFlagUsage)
	cmd.Flags().BoolVar(&allWords, "all-words", false, "Match all query words")
	cmd.Flags().BoolVar(&titlesOnly, "titles-only", false, "Search titles only")
	cmd.Flags().BoolVar(&openIssues, "open-issues", false, "Only return open issues")
	cmd.Flags().StringVar(&attachments, "attachments", "", "Attachment search: 0, 1, or only")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
