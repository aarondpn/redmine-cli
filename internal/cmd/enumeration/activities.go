package enumeration

import (
	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
)

// NewCmdActivities creates the activities command group.
func NewCmdActivities(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "activities",
		Aliases: []string{"activity"},
		Short:   "List time entry activities",
		Long: "List time entry activities. Without --project the shared (global) activities are " +
			"listed. With --project the activities available in that project are listed, " +
			"including project-specific overrides; override IDs differ from the global ones " +
			"and are the ones time entries in that project must use.",
	}
	cmd.AddCommand(newCmdActivitiesList(f))
	return cmd
}

func newCmdActivitiesList(f *cmdutil.Factory) *cobra.Command {
	var (
		project string
		format  string
	)
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Args:    cobra.NoArgs,
		Short:   "List time entry activities",
		Long: "List time entry activities. Without --project the shared (global) activities are " +
			"listed with their default and active flags. With --project the activities available " +
			"in that project (only active ones, project-specific overrides included) are listed " +
			"with ID and name only; this needs the view_time_entries permission on Redmine 6.x/7.x. " +
			"The default project is not applied here.",
		Example: `  # Global activities
  redmine activities list

  # Activities available in a project, including overrides
  redmine activities list --project myproject

  # ID of the default activity
  redmine activities list --jq '.[] | select(.is_default) | .id'`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmdutil.Context(cmd)
			// Explicit only: an implicit default project would silently change the
			// result from the global list to a project-specific one.
			projectID, err := cmdutil.ResolveProjectIdentifier(ctx, f, project)
			if err != nil {
				return err
			}
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)
			stop := printer.Spinner("Fetching activities...")
			result, err := ops.ListTimeEntryActivities(ctx, client, ops.ListTimeEntryActivitiesInput{ProjectID: projectID})
			stop()
			if err != nil {
				return err
			}
			renderEnumerations(printer, result.Activities, "activities", projectID == "")
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID (list the activities available in that project)")
	cmdutil.AddOutputFlag(cmd, &format)
	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	return cmd
}
