package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/api"
	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/tui"
)

// newCmdSearchBrowse creates the search browse command.
func newCmdSearchBrowse(f *cmdutil.Factory) *cobra.Command {
	var (
		project     string
		scope       string
		allWords    bool
		titlesOnly  bool
		openIssues  bool
		attachments string
		issues      bool
		news        bool
		documents   bool
		changesets  bool
		wikiPages   bool
		messages    bool
		projects    bool
	)

	cmd := &cobra.Command{
		Use:   "browse [query]",
		Short: "Interactive search result browser (TUI)",
		Long:  "Browse search results interactively with a split-screen detail view and live search.",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := cmdutil.PrepareInteractiveCommand(cmd, f); err != nil {
				return err
			}

			query := strings.Join(args, " ")

			project, err := cmdutil.DefaultProjectIdentifier(context.Background(), f, project)
			if err != nil {
				return err
			}

			cfg, err := f.Config()
			if err != nil {
				return err
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			// Build the base search params (everything except query).
			baseParams := api.SearchParams{
				ProjectID:   project,
				Scope:       scope,
				AllWords:    allWords,
				TitlesOnly:  titlesOnly,
				OpenIssues:  openIssues,
				Attachments: attachments,
				Issues:      issues,
				News:        news,
				Documents:   documents,
				Changesets:  changesets,
				WikiPages:   wikiPages,
				Messages:    messages,
				Projects:    projects,
				Limit:       100,
			}

			// Search function for the TUI to call on query changes.
			searchFn := func(q string) ([]models.SearchResult, error) {
				if q == "" {
					return nil, nil
				}
				p := baseParams
				p.Query = q
				results, _, err := client.Search.Search(context.Background(), p)
				if err != nil {
					return nil, fmt.Errorf("search failed: %w", err)
				}
				return results, nil
			}

			// Perform initial search if a query was provided.
			var initialResults []models.SearchResult
			if query != "" {
				printer := f.Printer("")
				stop := printer.Spinner("Searching...")
				initialResults, _, err = client.Search.Search(context.Background(), func() api.SearchParams {
					p := baseParams
					p.Query = query
					return p
				}())
				stop()
				if err != nil {
					return fmt.Errorf("search failed: %w", err)
				}
			}

			return tui.RunSearchBrowser(initialResults, query, cfg.Server, searchFn)
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Limit search to a project name, identifier, or ID")
	cmd.Flags().StringVar(&scope, "scope", "", "Search scope: all, my_projects, subprojects")
	cmd.Flags().BoolVar(&allWords, "all-words", false, "Match all query words")
	cmd.Flags().BoolVar(&titlesOnly, "titles-only", false, "Search titles only")
	cmd.Flags().BoolVar(&openIssues, "open-issues", false, "Only return open issues")
	cmd.Flags().StringVar(&attachments, "attachments", "", "Attachment search: 0, 1, or only")
	cmd.Flags().BoolVar(&issues, "issues", false, "Include issues in results")
	cmd.Flags().BoolVar(&news, "news", false, "Include news in results")
	cmd.Flags().BoolVar(&documents, "documents", false, "Include documents in results")
	cmd.Flags().BoolVar(&changesets, "changesets", false, "Include changesets in results")
	cmd.Flags().BoolVar(&wikiPages, "wiki-pages", false, "Include wiki pages in results")
	cmd.Flags().BoolVar(&messages, "messages", false, "Include forum messages in results")
	cmd.Flags().BoolVar(&projects, "projects", false, "Include projects in results")

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
