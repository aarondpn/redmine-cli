package membership

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdMembershipList(f *cmdutil.Factory) *cobra.Command {
	var (
		project string
		limit   int
		offset  int
		format  string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List project memberships",
		Long:    "List all memberships for a project.",
		Example: `  # List memberships for a project
  redmine memberships list --project myproject

  # Output as JSON
  redmine memberships list --project myproject -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			project, err = cmdutil.RequireProjectIdentifier(context.Background(), f, project)
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching memberships...")

			result, err := ops.ListMemberships(context.Background(), client, ops.ListMembershipsInput{
				ProjectID: project,
				Limit:     cmdutil.OpsLimit(limit),
				Offset:    offset,
			})
			stop()
			if err != nil {
				return fmt.Errorf("failed to list memberships: %s", cmdutil.FormatError(err))
			}
			memberships, total := result.Memberships, result.TotalCount

			if cmdutil.HandleEmpty(printer, memberships, "memberships") {
				return nil
			}

			cmdutil.RenderCollection(printer, memberships, []string{"ID", "User/Group", "Roles"},
				func(m models.Membership, styled bool) []string {
					id := strconv.Itoa(m.ID)
					if styled {
						id = output.StyleID.Render(id)
					}
					return []string{id, memberName(m), formatRoles(m.Roles)}
				},
			)

			cmdutil.WarnPagination(printer, cmdutil.PaginationResult{
				Shown: len(memberships), Total: total, Limit: limit, Offset: offset, Noun: "memberships",
			})

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID (required)")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}

func memberName(m models.Membership) string {
	if m.User != nil {
		return m.User.Name
	}
	if m.Group != nil {
		return m.Group.Name + " (group)"
	}
	return "unknown"
}

func formatRoles(roles []models.IDName) string {
	names := make([]string, len(roles))
	for i, r := range roles {
		names[i] = r.Name
	}
	return strings.Join(names, ", ")
}
