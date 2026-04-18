package version

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
	"github.com/aarondpn/redmine-cli/v2/internal/resolver"
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

			ctx := context.Background()

			var id int
			if numID, err := strconv.Atoi(args[0]); err == nil {
				id = numID
			} else {
				// Non-numeric: resolve by name, requires project
				project = cmdutil.DefaultProject(f, project)
				if project == "" {
					return fmt.Errorf("--project is required when looking up a version by name")
				}
				project, err = cmdutil.ResolveProjectIdentifier(ctx, f, project)
				if err != nil {
					return err
				}
				resolved, err := resolver.ResolveVersion(ctx, client, args[0], project)
				if err != nil {
					return err
				}
				id = resolved
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching version...")
			version, err := client.Versions.Get(ctx, id)
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
