package version

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdVersionGet(f *cmdutil.Factory) *cobra.Command {
	var (
		format  string
		project string
	)

	cmd := &cobra.Command{
		Use:     "get <id-or-name>",
		Aliases: []string{"show", "view"},
		Short:   "Get version details",
		Long:    "Display detailed information about a specific version. Accepts a numeric ID or version name (uses the default project, or pass --project).",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := cmdutil.Context(cmd)
			id, err := resolveVersionID(ctx, f, client, args[0], project)
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching version...")
			version, err := ops.GetVersion(ctx, client, ops.GetVersionInput{ID: id})
			stop()
			if err != nil {
				return fmt.Errorf("failed to get version %d: %w", id, err)
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(version)
				return nil
			}

			pairs := []output.KeyValue{
				{Key: "ID", Value: output.StyleID.Render(fmt.Sprintf("%d", version.ID))},
				{Key: "Project", Value: version.Project.Name},
				{Key: "Name", Value: version.Name},
				{Key: "Status", Value: output.StatusStyle(version.Status).Render(version.Status)},
				{Key: "Sharing", Value: version.Sharing},
			}

			if version.DueDate != "" {
				pairs = append(pairs, output.KeyValue{Key: "Due Date", Value: version.DueDate})
			}
			if version.Description != "" {
				pairs = append(pairs, output.KeyValue{Key: "Description", Value: version.Description})
			}

			if version.WikiPageTitle != "" {
				pairs = append(pairs, output.KeyValue{Key: "Wiki Page", Value: version.WikiPageTitle})
			}
			if version.EstimatedHours != nil {
				pairs = append(pairs, output.KeyValue{Key: "Estimated Hours", Value: cmdutil.FormatHours(*version.EstimatedHours)})
			}
			if version.SpentHours != nil {
				pairs = append(pairs, output.KeyValue{Key: "Spent Hours", Value: cmdutil.FormatHours(*version.SpentHours)})
			}
			if cf := cmdutil.FormatCustomFields(version.CustomFields); cf != "" {
				pairs = append(pairs, output.KeyValue{Key: "Custom Fields", Value: cf})
			}

			pairs = append(pairs,
				output.KeyValue{Key: "Created", Value: version.CreatedOn},
				output.KeyValue{Key: "Updated", Value: version.UpdatedOn},
			)

			printer.Detail(pairs)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID (for name resolution; falls back to default project)")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
