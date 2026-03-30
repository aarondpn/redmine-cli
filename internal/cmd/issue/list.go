package issue

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/output"
	"github.com/aarondpn/redmine-cli/internal/resolver"
)

// NewCmdList creates the issues list command.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	var (
		project     string
		tracker     string
		status      string
		assignee    string
		version     string
		sort        string
		include     string
		attachments bool
		relations   bool
		limit       int
		offset      int
		format      string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List issues",
		Long:    "List issues with optional filters for project, tracker, status, and assignee.",
		Example: `  # List open issues for a project
  redmine issues list --project myproject

  # List ALL issues with no limit
  redmine issues list --project myproject --limit 0

  # Page through issues
  redmine issues list --project myproject --limit 25 --offset 0
  redmine issues list --project myproject --limit 25 --offset 25

  # Filter by version (name or ID) and output as JSON
  redmine issues list --project myproject --version "v1.0" -o json
  redmine issues list --version 42 -o json

  # Closed issues assigned to me, sorted by update date
  redmine issues list --status closed --assignee me --sort updated_on:desc

  # All issues regardless of status
  redmine issues list --project myproject --status "*" --limit 0 -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			project = cmdutil.DefaultProject(f, project)

			if project != "" {
				project, err = cmdutil.ResolveProjectID(context.Background(), f, project)
				if err != nil {
					return err
				}
			}

			var trackerID int
			if tracker != "" {
				trackerID, err = resolver.ResolveTracker(context.Background(), client, tracker)
				if err != nil {
					return fmt.Errorf("resolving tracker: %w", err)
				}
			}

			var versionID int
			if version != "" {
				id, err := resolver.ResolveVersion(context.Background(), client, version, project)
				if err != nil {
					return fmt.Errorf("resolving version: %w", err)
				}
				versionID = id
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

			filter := models.IssueFilter{
				ProjectID:      project,
				TrackerID:      trackerID,
				StatusID:       status,
				AssignedToID:   assignee,
				FixedVersionID: versionID,
				Sort:           sort,
				Includes:       includes,
				Limit:          limit,
				Offset:         offset,
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching issues...")
			issues, total, err := client.Issues.List(context.Background(), filter)
			stop()
			if err != nil {
				return fmt.Errorf("failed to list issues: %w", err)
			}

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
	cmd.Flags().StringVar(&status, "status", "open", "Status filter: open, closed, *, or status ID")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee ID or 'me'")
	cmd.Flags().StringVar(&version, "version", "", "Filter by version name or ID")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort field (e.g., updated_on:desc)")
	cmd.Flags().StringVar(&include, "include", "", "Include related data: attachments,relations")
	cmd.Flags().BoolVar(&attachments, "attachments", false, "Include attachments (shorthand for --include attachments)")
	cmd.Flags().BoolVar(&relations, "relations", false, "Include issue relations (shorthand for --include relations)")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("tracker", cmdutil.CompleteTrackers(f))
	_ = cmd.RegisterFlagCompletionFunc("status", cmdutil.CompleteIssueListStatus(f))
	_ = cmd.RegisterFlagCompletionFunc("assignee", cmdutil.CompleteUsers(f))
	_ = cmd.RegisterFlagCompletionFunc("version", cmdutil.CompleteVersions(f))

	return cmd
}
