package search

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/api"
	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// Help text shared by every search command that accepts --project.
const (
	defaultProjectHelp = "If a default project is configured and --project is not given, the search is\n" +
		"limited to the default project and a notice is printed to stderr. Use --scope all\n" +
		"to search all projects, or --project to pick one explicitly."
	projectFlagUsage = "Limit search to a project name, identifier, or ID (defaults to the configured default project)"
	scopeFlagUsage   = "Search scope: all, my_projects, bookmarks, subprojects (all, my_projects and bookmarks ignore the default project)"
)

// resolveSearchProject applies the default-project fallback shared by the
// search commands and resolves it to the canonical identifier. Searching is
// commonly expected to be global, so when the default project is applied
// implicitly it also returns a one-line notice for printDefaultProjectNotice.
// There is no notice when --project was given explicitly or when --scope makes
// Redmine ignore the project (all, my_projects, bookmarks).
func resolveSearchProject(ctx context.Context, f *cmdutil.Factory, project, scope string) (identifier, notice string, err error) {
	identifier, err = cmdutil.DefaultProjectIdentifier(ctx, f, project)
	if err != nil {
		return "", "", err
	}
	if project == "" && identifier != "" {
		notice = defaultProjectNotice(identifier, scope)
	}
	return identifier, notice, nil
}

// defaultProjectNotice returns the notice for an implicitly applied default
// project, or "" when the scope makes Redmine ignore the project. Any other
// scope value (including unknown ones, for which Redmine falls back to the
// project) keeps the default project in effect.
func defaultProjectNotice(identifier, scope string) string {
	switch scope {
	case "all", "my_projects", "bookmarks":
		return ""
	case "subprojects":
		return fmt.Sprintf("Searching project %q and its subprojects (default project). Use --scope all to search all projects.", identifier)
	default:
		return fmt.Sprintf("Searching project %q (default project). Use --scope all to search all projects.", identifier)
	}
}

// printDefaultProjectNotice emits the default-project notice after a
// successful search, so a failed search reports only its error.
//
// Unlike most warnings it is not gated on output.SupportsWarnings: it goes
// to stderr, so JSON on stdout stays clean, and JSON callers (mostly agents)
// are exactly the audience that otherwise misses the narrowed scope.
func printDefaultProjectNotice(printer output.Printer, notice string) {
	if notice != "" {
		printer.Warning(notice)
	}
}

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
		Long: "Search for issues, news, documents, wiki pages, messages, and projects in Redmine.\n\n" +
			defaultProjectHelp + "\n\n" +
			"Without type flags, Redmine searches every resource type (including changesets);\n" +
			"pass --issues, --wiki-pages, etc. to narrow the results.",
		Example: `  # Search issue titles only (in the default project, if one is configured)
  redmine search "login timeout" --issues --titles-only

  # Search every project you can see, not just the default project
  redmine search "login timeout" --scope all --issues

  # Search a specific project
  redmine search "deploy checklist" --project myproject --wiki-pages`,
		Args: cobra.MinimumNArgs(1),
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
			results, total, err := client.Search.Search(cmdutil.Context(cmd), params)
			stop()
			if err != nil {
				return fmt.Errorf("search failed: %w", err)
			}
			printDefaultProjectNotice(printer, notice)

			if cmdutil.HandleEmpty(printer, results, "results") {
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
func typeStyle(t string) output.Style {
	switch strings.ToLower(t) {
	case "issue":
		return output.NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("12")))
	case "issue-closed":
		return output.NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("8")))
	case "news":
		return output.NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("11")))
	case "wiki-page":
		return output.NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("13")))
	case "changeset":
		return output.NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("10")))
	case "message":
		return output.NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("14")))
	case "project":
		return output.NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("6")))
	case "document":
		return output.NewStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("3")))
	default:
		return output.NewStyle(lipgloss.NewStyle())
	}
}

// formatDate truncates an ISO datetime to just the date portion.
func formatDate(datetime string) string {
	if len(datetime) >= 10 {
		return datetime[:10]
	}
	return datetime
}
