package project

import (
	"context"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdList(f *cmdutil.Factory) *cobra.Command {
	var (
		limit    int
		offset   int
		format   string
		includes []string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List projects",
		Long:    "List all accessible Redmine projects.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateProjectIncludes(includes); err != nil {
				return err
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			result, err := ops.ListProjects(context.Background(), client, ops.ListProjectsInput{
				Limit:    cmdutil.OpsLimit(limit),
				Offset:   offset,
				Includes: includes,
			})
			if err != nil {
				return err
			}
			projects, total := result.Projects, result.TotalCount

			if cmdutil.HandleEmpty(printer, projects, "projects") {
				return nil
			}

			cmdutil.RenderCollection(printer, projects, []string{"ID", "Identifier", "Name", "Status", "Public"},
				func(p models.Project, styled bool) []string {
					id := strconv.Itoa(p.ID)
					if styled {
						id = output.StyleID.Render(id)
					}
					return []string{id, p.Identifier, p.Name, projectStatusLabel(p.Status), formatBool(p.IsPublic)}
				},
			)

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(projects), Total: total, Limit: limit, Offset: offset, Noun: "projects",
			})

			return nil
		},
	}

	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)
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
		return "archived"
	case 9:
		return "closed"
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
