package version

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
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
	cmd.AddCommand(newCmdVersionGet(f))
	return cmd
}

func newCmdVersionList(f *cmdutil.Factory) *cobra.Command {
	var (
		project      string
		statusFilter string
		open         bool
		closed       bool
		locked       bool
		limit        int
		offset       int
		format       string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List project versions",
		Long:    "List all versions for a project, optionally filtered by status.",
		Example: `  # List all versions
  redmine versions list --project myproject

  # List ALL versions with no limit
  redmine versions list --project myproject --limit 0

  # Page through versions
  redmine versions list --project myproject --limit 25 --offset 0
  redmine versions list --project myproject --limit 25 --offset 25

  # Filter by status and output as JSON
  redmine versions list --project myproject --open -o json

  # Output as CSV
  redmine versions list --project myproject --closed -o csv`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			// Resolve shorthand status flags
			if open {
				statusFilter = "open"
			} else if closed {
				statusFilter = "closed"
			} else if locked {
				statusFilter = "locked"
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

			var versions []models.Version
			var hasMore bool
			if statusFilter != "" {
				// Page through the API, collecting only matching versions
				// until we have enough for offset + limit.
				need := 0
				if limit > 0 {
					need = offset + limit
				}
				matched, more, err := client.Versions.ListFiltered(
					context.Background(), project, need,
					func(v models.Version) bool { return v.Status == statusFilter },
				)
				stop()
				if err != nil {
					return fmt.Errorf("failed to list versions: %s", cmdutil.FormatError(err))
				}
				versions = matched
				hasMore = more

				// Apply client-side offset to filtered results
				if offset > 0 && offset < len(versions) {
					versions = versions[offset:]
				} else if offset >= len(versions) {
					versions = nil
				}
			} else {
				fetched, total, err := client.Versions.List(context.Background(), project, limit, offset)
				stop()
				if err != nil {
					return fmt.Errorf("failed to list versions: %s", cmdutil.FormatError(err))
				}
				versions = fetched
				hasMore = limit > 0 && total > limit+offset
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

			if hasMore {
				printer.Warning("More versions available. Use --limit and --offset to paginate.")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project identifier (required)")
	cmd.Flags().StringVar(&statusFilter, "status", "", "Filter by status: open, locked, closed")
	cmd.Flags().BoolVar(&open, "open", false, "Show only open versions")
	cmd.Flags().BoolVar(&closed, "closed", false, "Show only closed versions")
	cmd.Flags().BoolVar(&locked, "locked", false, "Show only locked versions")
	cmd.MarkFlagsMutuallyExclusive("open", "closed", "locked")
	cmd.MarkFlagsMutuallyExclusive("open", "status")
	cmd.MarkFlagsMutuallyExclusive("closed", "status")
	cmd.MarkFlagsMutuallyExclusive("locked", "status")
	cmdutil.AddPaginationFlags(cmd, &limit, &offset)
	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}
