package project

import (
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdList(f *cmdutil.Factory) *cobra.Command {
	var (
		status      string
		sort        string
		extraFilter []string
		limit       int
		offset      int
		format      string
		includes    []string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Args:    cobra.NoArgs,
		Aliases: []string{"ls"},
		Short:   "List projects",
		Long: `List accessible Redmine projects.

Redmine 4.1+ lists only active projects unless --status is given. Archived
projects are never listed by the REST API, not even for administrators.`,
		Example: `  # Active and closed projects
  redmine projects list --status all

  # Only closed projects, newest first
  redmine projects list --status closed --sort created_on:desc

  # Raw filter escape hatch: name contains "web", created this year
  redmine projects list --filter name='~web' --filter created_on='>=2026-01-01'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateProjectIncludes(includes); err != nil {
				return err
			}
			if _, err := ops.ProjectStatusFilter(status); err != nil {
				return err
			}
			extra, err := cmdutil.ParseFilterFlag(extraFilter)
			if err != nil {
				return err
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			result, err := ops.ListProjects(cmdutil.Context(cmd), client, ops.ListProjectsInput{
				Status:      status,
				Sort:        sort,
				ExtraParams: extra,
				Limit:       cmdutil.OpsLimit(limit),
				Offset:      offset,
				Includes:    includes,
			})
			if err != nil {
				return err
			}
			projects, total := result.Projects, result.TotalCount

			if cmdutil.HandleEmpty(printer, projects, "projects") {
				return nil
			}

			cmdutil.RenderCollection(printer, projects, []string{"ID", "Identifier", "Name", "Status", "Public"},
				func(p models.Project) []string {
					id := output.StyleID.Render(strconv.Itoa(p.ID))
					return []string{id, p.Identifier, p.Name, projectStatusLabel(p.Status), formatBool(p.IsPublic)}
				},
			)

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(projects), Total: total, Limit: limit, Offset: offset, Noun: "projects",
			})

			return nil
		},
	}

	cmd.Flags().StringVar(&status, "status", "", "Filter by status: active, closed, all, or a numeric status (default: Redmine lists active projects only)")
	cmd.Flags().StringVar(&sort, "sort", "", "Sort field (e.g., name or created_on:desc)")
	cmdutil.AddFilterFlag(cmd, &extraFilter, "name='~web'", "created_on='>=2026-01-01'")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)
	_ = cmd.RegisterFlagCompletionFunc("status", cmdutil.CompleteProjectListStatus)
	cmd.Flags().StringSliceVar(&includes, "include", nil,
		"Include related data: trackers, issue_categories, enabled_modules, time_entry_activities, issue_custom_fields (repeatable or comma-separated)")
	_ = cmd.RegisterFlagCompletionFunc("include", completeProjectIncludes)

	return cmd
}

func projectStatusLabel(status int) string {
	switch status {
	case 1:
		return "active"
	case 5:
		return "closed"
	case 9:
		return "archived"
	default:
		return strconv.Itoa(status)
	}
}

func formatBool(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
