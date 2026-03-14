package issue

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/output"
)

// NewCmdList creates the issues list command.
func NewCmdList(f *cmdutil.Factory) *cobra.Command {
	var (
		project  string
		tracker  int
		status   string
		assignee string
		version  string
		sort     string
		limit    int
		offset   int
		format   string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List issues",
		Long:    "List issues with optional filters for project, tracker, status, and assignee.",
		RunE: func(cmd *cobra.Command, args []string) error {
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

			var versionID int
			if version != "" {
				if id, err := strconv.Atoi(version); err == nil {
					versionID = id
				} else {
					if project == "" {
						return fmt.Errorf("--project is required when filtering by version name")
					}
					versions, _, err := client.Versions.List(context.Background(), project, 0)
					if err != nil {
						return fmt.Errorf("failed to fetch versions for name resolution: %s", cmdutil.FormatError(err))
					}
					needle := strings.ToLower(version)
					var matches []models.Version
					for _, v := range versions {
						if strings.ToLower(v.Name) == needle {
							matches = append(matches, v)
						}
					}
					if len(matches) == 0 {
						names := make([]string, len(versions))
						for i, v := range versions {
							names[i] = fmt.Sprintf("  - %s (ID: %d)", v.Name, v.ID)
						}
						return fmt.Errorf("no version found matching %q. Available versions:\n%s", version, strings.Join(names, "\n"))
					}
					if len(matches) > 1 {
						return fmt.Errorf("multiple versions match %q, please use the version ID instead", version)
					}
					versionID = matches[0].ID
				}
			}

			filter := models.IssueFilter{
				ProjectID:      project,
				TrackerID:      tracker,
				StatusID:       status,
				AssignedToID:   assignee,
				FixedVersionID: versionID,
				Sort:           sort,
				Limit:          limit,
				Offset:         offset,
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching issues...")
			issues, total, err := client.Issues.List(context.Background(), filter)
			stop()
			if err != nil {
				return fmt.Errorf("failed to list issues: %s", cmdutil.FormatError(err))
			}

			if len(issues) == 0 {
				printer.Warning("No issues found")
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

			if total > limit+offset {
				printer.Warning(fmt.Sprintf("Showing %d of %d issues. Use --offset to paginate.", len(issues), total))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier")
	cmd.Flags().IntVar(&tracker, "tracker", 0, "Tracker ID")
	cmd.Flags().StringVar(&status, "status", "open", "Status filter: open, closed, *, or status ID")
	cmd.Flags().StringVar(&assignee, "assignee", "", "Assignee ID or 'me'")
	cmd.Flags().StringVar(&version, "version", "", "Filter by version name or ID")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort field (e.g., updated_on:desc)")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}
