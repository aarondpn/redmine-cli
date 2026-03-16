package version

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
)

func newCmdVersionGet(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "get <id>",
		Aliases: []string{"show", "view"},
		Short:   "Get version details",
		Long:    "Display detailed information about a specific version.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid version ID: %s", args[0])
			}

			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching version...")
			version, err := client.Versions.Get(context.Background(), id)
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

	cmdutil.AddOutputFlag(cmd, &format)
	return cmd
}
