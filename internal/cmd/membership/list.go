package membership

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

			if project == "" {
				cfg, err := f.Config()
				if err == nil && cfg.DefaultProject != "" {
					project = cfg.DefaultProject
				}
			}
			if project == "" {
				return fmt.Errorf("project is required. Use --project or set a default project")
			}

			project, err = cmdutil.ResolveProjectIdentifier(context.Background(), f, project)
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching memberships...")

			memberships, total, err := client.Memberships.List(context.Background(), project, limit, offset)
			stop()
			if err != nil {
				return fmt.Errorf("failed to list memberships: %s", cmdutil.FormatError(err))
			}

			if len(memberships) == 0 {
				if printer.Format() == output.FormatJSON {
					printer.JSON(memberships)
					return nil
				}
				if output.SupportsWarnings(printer.Format()) {
					printer.Warning("No memberships found")
				}
				return nil
			}

			switch printer.Format() {
			case output.FormatJSON:
				printer.JSON(memberships)
			case output.FormatCSV:
				headers := []string{"ID", "User/Group", "Roles"}
				rows := make([][]string, len(memberships))
				for i, m := range memberships {
					rows[i] = []string{
						fmt.Sprintf("%d", m.ID),
						memberName(m),
						formatRoles(m.Roles),
					}
				}
				printer.CSV(headers, rows)
			default:
				headers := []string{"ID", "User/Group", "Roles"}
				rows := make([][]string, len(memberships))
				for i, m := range memberships {
					rows[i] = []string{
						output.StyleID.Render(strconv.Itoa(m.ID)),
						memberName(m),
						formatRoles(m.Roles),
					}
				}
				printer.Table(headers, rows)
			}

			if limit > 0 && total > limit+offset {
				if output.SupportsWarnings(printer.Format()) {
					printer.Warning(fmt.Sprintf("Showing %d of %d memberships. Use --limit and --offset to paginate.", len(memberships), total))
				}
			}

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
