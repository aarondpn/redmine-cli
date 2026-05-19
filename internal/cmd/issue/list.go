package issue

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
)

// NewCmdList creates the issues list command.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	var (
		project            string
		tracker            string
		status             string
		assignee           string
		author             string
		priority           string
		category           string
		version            string
		parent             int
		subproject         string
		includeSubprojects bool
		isPrivate          bool
		query              string
		queryID            int
		sort               string
		include            string
		attachments        bool
		relations          bool
		extraFilter        []string
		limit              int
		offset             int
		format             string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List issues",
		Long:    "List issues with filters. Combine flags for narrow searches.",
		Example: `  # List open issues for a project
  redmine issues list --project myproject

  # List ALL issues with no limit
  redmine issues list --project myproject --limit 0

  # Filter by author and priority
  redmine issues list --project myproject --author me --priority High

  # Filter by parent issue and category
  redmine issues list --project myproject --parent 1234 --category "Backend"

  # Exclude subprojects
  redmine issues list --project myproject --subproject "!*"

  # Closed issues assigned to me, sorted by update date
  redmine issues list --status closed --assignee me --sort updated_on:desc

  # Raw filter escape hatch: date ranges, custom fields, subject text
  redmine issues list --project myproject --filter created_on='>=2025-01-01'
  redmine issues list --project myproject --filter cf_5='Critical'
  redmine issues list --project myproject --filter subject='~login'

  # Run a saved query by name or ID
  redmine issues list --query "My open issues"
  redmine issues list --query-id 12`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := context.Background()
			printer := f.Printer(format)

			project, err = cmdutil.DefaultProjectID(ctx, f, project)
			if err != nil {
				return err
			}

			var trackerID int
			if tracker != "" {
				trackerID, err = resolver.ResolveTracker(ctx, client, tracker)
				if err != nil {
					return fmt.Errorf("resolving tracker: %w", err)
				}
			}

			resolvedStatus, err := resolveIssueStatusFilter(ctx, client, status)
			if err != nil {
				return err
			}

			resolvedAssignee, err := resolveIssueAssigneeFilter(ctx, client, assignee, printer)
			if err != nil {
				return err
			}

			resolvedAuthor, err := resolveIssueAuthorFilter(ctx, client, author, printer)
			if err != nil {
				return err
			}

			resolvedPriority, err := resolveIssuePriorityFilter(ctx, client, priority)
			if err != nil {
				return err
			}

			var categoryID int
			if category != "" {
				id, err := resolver.ResolveCategory(ctx, client, category, project)
				if err != nil {
					return fmt.Errorf("resolving category: %w", err)
				}
				categoryID = id
			}

			var versionID int
			if version != "" {
				id, err := resolver.ResolveVersion(ctx, client, version, project)
				if err != nil {
					return fmt.Errorf("resolving version: %w", err)
				}
				versionID = id
			}

			// MarkFlagsMutuallyExclusive (below) catches `--query foo --query-id N`,
			// but it relies on cobra's `Changed` bookkeeping. We thread the same
			// signal into the resolver so a lone `--query-id 0` (which leaves
			// IntVar at its zero value) is rejected explicitly instead of being
			// silently treated as "no filter".
			resolvedQueryID, err := resolveIssueQueryFilter(ctx, client, query, queryID, cmd.Flags().Changed("query-id"), project)
			if err != nil {
				return err
			}

			var includes []string
			if include != "" {
				includes = strings.Split(include, ",")
			}
			if attachments {
				includes = append(includes, "attachments")
			}
			if relations {
				includes = append(includes, "relations")
			}

			var isPrivatePtr *bool
			if cmd.Flags().Changed("is-private") {
				v := isPrivate
				isPrivatePtr = &v
			}

			subprojectFilter := subproject
			if cmd.Flags().Changed("include-subprojects") && !includeSubprojects && subprojectFilter == "" {
				subprojectFilter = "!*"
			}

			extra, err := parseExtraFilters(extraFilter)
			if err != nil {
				return err
			}

			stop := printer.Spinner("Fetching issues...")
			result, err := ops.ListIssues(ctx, client, ops.ListIssuesInput{
				ProjectID:      project,
				SubprojectID:   subprojectFilter,
				TrackerID:      trackerID,
				StatusID:       resolvedStatus,
				AssignedToID:   resolvedAssignee,
				AuthorID:       resolvedAuthor,
				PriorityID:     resolvedPriority,
				CategoryID:     categoryID,
				FixedVersionID: versionID,
				ParentID:       parent,
				IsPrivate:      isPrivatePtr,
				QueryID:        resolvedQueryID,
				Sort:           sort,
				Includes:       includes,
				ExtraParams:    extra,
				Limit:          cmdutil.OpsLimit(limit),
				Offset:         offset,
			})
			stop()
			if err != nil {
				return fmt.Errorf("failed to list issues: %w", err)
			}
			issues, total := result.Issues, result.TotalCount

			if cmdutil.HandleEmpty(printer, issues, "issues") {
				return nil
			}

			switch printer.Format() {
			case output.FormatJSON:
				printer.JSON(issues)
			case output.FormatCSV:
				headers := []string{"ID", "Tracker", "Status", "Priority", "Subject", "Assignee", "Version"}
				rows := make([][]string, len(issues))
				for i, issue := range issues {
					rows[i] = []string{
						fmt.Sprintf("#%d", issue.ID),
						issue.Tracker.Name,
						issue.Status.Name,
						issue.Priority.Name,
						issue.Subject,
						assigneeName(issue.AssignedTo),
						assigneeName(issue.FixedVersion),
					}
				}
				printer.CSV(headers, rows)
			default:
				headers := []string{"ID", "Tracker", "Status", "Priority", "Subject", "Assignee", "Version"}
				rows := make([][]string, len(issues))
				for i, issue := range issues {
					rows[i] = []string{
						output.StyleID.Render(fmt.Sprintf("#%d", issue.ID)),
						issue.Tracker.Name,
						output.StatusStyle(issue.Status.Name).Render(issue.Status.Name),
						output.PriorityStyle(issue.Priority.Name).Render(issue.Priority.Name),
						issue.Subject,
						assigneeName(issue.AssignedTo),
						assigneeName(issue.FixedVersion),
					}
				}
				printer.Table(headers, rows)
			}

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(issues), Total: total, Limit: limit, Offset: offset, Noun: "issues",
			})

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID")
	cmd.Flags().StringVar(&tracker, "tracker", "", "Tracker name or ID")
	cmd.Flags().StringVar(&status, "status", "open", "Status filter: open, closed, *, status name, or ID")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee ID, name, login, or 'me'")
	cmd.Flags().StringVar(&author, "author", "", "Author ID, name, login, or 'me'")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority name or ID")
	cmd.Flags().StringVar(&category, "category", "", "Issue category name or ID")
	cmd.Flags().StringVar(&version, "version", "", "Filter by version name or ID")
	cmd.Flags().IntVar(&parent, "parent", 0, "Restrict to children of this parent issue ID")
	cmd.Flags().StringVar(&subproject, "subproject", "", "Subproject filter (numeric ID, or '!*' to exclude subprojects)")
	cmd.Flags().BoolVar(&includeSubprojects, "include-subprojects", true, "Include issues from subprojects (set to false to exclude)")
	cmd.Flags().BoolVar(&isPrivate, "is-private", false, "Filter by privacy: true returns only private, false returns only public issues")
	cmd.Flags().StringVar(&query, "query", "", "Run a saved query by name (mutually exclusive with --query-id)")
	cmd.Flags().IntVar(&queryID, "query-id", 0, "Run a saved query by numeric ID")
	cmd.MarkFlagsMutuallyExclusive("query", "query-id")
	cmd.MarkFlagsMutuallyExclusive("subproject", "include-subprojects")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort field (e.g., updated_on:desc)")
	cmd.Flags().StringVar(&include, "include", "", "Include related data: attachments,relations")
	cmd.Flags().BoolVar(&attachments, "attachments", false, "Include attachments (shorthand for --include attachments)")
	cmd.Flags().BoolVar(&relations, "relations", false, "Include issue relations (shorthand for --include relations)")
	cmd.Flags().StringArrayVar(&extraFilter, "filter", nil, "Raw Redmine filter as key=value (repeatable). Examples: --filter created_on='>=2025-01-01', --filter cf_5=Critical, --filter subject='~login'")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("tracker", cmdutil.CompleteTrackers(f))
	_ = cmd.RegisterFlagCompletionFunc("status", cmdutil.CompleteIssueListStatus(f))
	_ = cmd.RegisterFlagCompletionFunc("assignee", cmdutil.CompleteUsers(f))
	_ = cmd.RegisterFlagCompletionFunc("author", cmdutil.CompleteUsers(f))
	_ = cmd.RegisterFlagCompletionFunc("priority", cmdutil.CompletePriorities(f))
	_ = cmd.RegisterFlagCompletionFunc("category", cmdutil.CompleteCategories(f))
	_ = cmd.RegisterFlagCompletionFunc("version", cmdutil.CompleteVersions(f))

	return cmd
}

// parseExtraFilters parses repeated --filter key=value entries into a map.
// Keys are kept verbatim; values are passed through unchanged so callers can
// use Redmine's operator syntax (>=YYYY-MM-DD, ><a|b, ~text, etc).
func parseExtraFilters(raws []string) (map[string]string, error) {
	if len(raws) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(raws))
	for _, raw := range raws {
		key, val, ok := strings.Cut(raw, "=")
		if !ok {
			return nil, fmt.Errorf("--filter %q must be key=value", raw)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, fmt.Errorf("--filter %q: empty key", raw)
		}
		out[key] = val
	}
	return out, nil
}
