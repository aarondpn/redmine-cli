package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/api"
	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/output"
)

// NewCmdSearch creates the parent search command with general search functionality.
func NewCmdSearch(f *cmdutil.Factory) *cobra.Command {
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
		limit       int
		offset      int
		format      string
	)

	cmd := &cobra.Command{
		Use:   "search <query>",
		Short: "Search across Redmine resources",
		Long:  "Search for issues, news, documents, wiki pages, messages, and projects in Redmine.",
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
				Issues:      issues,
				News:        news,
				Documents:   documents,
				Changesets:  changesets,
				WikiPages:   wikiPages,
				Messages:    messages,
				Projects:    projects,
				Limit:       limit,
				Offset:      offset,
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Searching...")
			results, total, err := client.Search.Search(context.Background(), params)
			stop()
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}

			if cmdutil.HandleEmpty(printer, results, "results") {
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
	cmd.Flags().BoolVar(&openIssues, "open-issues", false, "Only return open issues")
	cmd.Flags().StringVar(&attachments, "attachments", "", "Attachment search: 0 (description only), 1 (description+attachments), only (attachments only)")
	cmd.Flags().BoolVar(&issues, "issues", false, "Include issues in results")
	cmd.Flags().BoolVar(&news, "news", false, "Include news in results")
	cmd.Flags().BoolVar(&documents, "documents", false, "Include documents in results")
	cmd.Flags().BoolVar(&changesets, "changesets", false, "Include changesets in results")
	cmd.Flags().BoolVar(&wikiPages, "wiki-pages", false, "Include wiki pages in results")
	cmd.Flags().BoolVar(&messages, "messages", false, "Include forum messages in results")
	cmd.Flags().BoolVar(&projects, "projects", false, "Include projects in results")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	// Subcommands for dedicated resource search
	cmd.AddCommand(newCmdSearchIssues(f))
	cmd.AddCommand(newCmdSearchWiki(f))
	cmd.AddCommand(newCmdSearchProjects(f))
	cmd.AddCommand(newCmdSearchNews(f))
	cmd.AddCommand(newCmdSearchMessages(f))
	cmd.AddCommand(newCmdSearchBrowse(f))

	return cmd
}

// printResults renders search results using the printer.
func printResults(printer output.Printer, results []models.SearchResult, total, limit, offset int) {
	cmdutil.RenderCollection(printer, results, []string{"ID", "Type", "Title", "Date"}, func(r models.SearchResult, styled bool) []string {
		id := fmt.Sprintf("%d", r.ID)
		typ := r.Type
		date := r.DateTime
		if styled {
			id = output.StyleID.Render(fmt.Sprintf("%d", r.ID))
			typ = typeStyle(r.Type).Render(r.Type)
			date = formatDate(r.DateTime)
		}
		return []string{id, typ, r.Title, date}
	})

	cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
		Shown: len(results), Total: total, Limit: limit, Offset: offset, Noun: "results",
	})
}

// typeStyle returns a color style based on the result type.
func typeStyle(t string) lipgloss.Style {
	switch strings.ToLower(t) {
	case "issue":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
	case "issue-closed":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	case "news":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	case "wiki-page":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	case "changeset":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	case "message":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	case "project":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("6"))
	case "document":
		return lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
	default:
		return lipgloss.NewStyle()
	}
}

// formatDate truncates an ISO datetime to just the date portion.
func formatDate(datetime string) string {
	if len(datetime) >= 10 {
		return datetime[:10]
	}
	return datetime
}
