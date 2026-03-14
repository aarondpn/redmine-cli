package version

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
)

// NewCmdVersions creates the versions command group.
func NewCmdVersions(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "versions",
		Aliases: []string{"v"},
		Short:   "Manage project versions",
		Long:    "List and view Redmine project versions (milestones).",
	}

	cmd.AddCommand(newCmdVersionList(f))
	return cmd
}

func newCmdVersionList(f *cmdutil.Factory) *cobra.Command {
	var (
		project      string
		statusFilter string
		format       string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List project versions",
		Long:    "List all versions for a project, optionally filtered by status.",
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

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching versions...")
			versions, _, err := client.Versions.List(context.Background(), project, 0)
			stop()
			if err != nil {
				return fmt.Errorf("failed to list versions: %s", cmdutil.FormatError(err))
			}

			// Client-side status filter
			if statusFilter != "" {
				filtered := versions[:0]
				for _, v := range versions {
					if v.Status == statusFilter {
						filtered = append(filtered, v)
					}
				}
				versions = filtered
			}

			if len(versions) == 0 {
				printer.Warning("No versions found")
				return nil
			}

			switch printer.Format() {
			case output.FormatJSON:
				printer.JSON(versions)
			case output.FormatCSV:
				headers := []string{"ID", "Name", "Status", "Due Date", "Sharing", "Description"}
				rows := make([][]string, len(versions))
				for i, v := range versions {
					rows[i] = []string{
						fmt.Sprintf("%d", v.ID),
						v.Name,
						v.Status,
						v.DueDate,
						v.Sharing,
						v.Description,
					}
				}
				printer.CSV(headers, rows)
			default:
				headers := []string{"ID", "Name", "Status", "Due Date", "Sharing", "Description"}
				rows := make([][]string, len(versions))
				for i, v := range versions {
					rows[i] = []string{
						output.StyleID.Render(fmt.Sprintf("%d", v.ID)),
						v.Name,
						output.StatusStyle(v.Status).Render(v.Status),
						v.DueDate,
						v.Sharing,
						v.Description,
					}
				}
				printer.Table(headers, rows)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier (required)")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status: open, locked, closed")
	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}
