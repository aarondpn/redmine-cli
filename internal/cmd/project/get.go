package project

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdGet(f *cmdutil.Factory) *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "get <identifier>",
		Aliases: []string{"show", "view"},
		Short:   "Get project details",
		Long:    "Display detailed information about a Redmine project.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			project, err := ops.GetProject(context.Background(), client, ops.GetProjectInput{
				Identifier: args[0],
			})
			if err != nil {
				return err
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(project)
				return nil
			}

			pairs := []output.KeyValue{
				{Key: "ID", Value: output.StyleID.Render(strconv.Itoa(project.ID))},
				{Key: "Name", Value: project.Name},
				{Key: "Identifier", Value: project.Identifier},
				{Key: "Description", Value: project.Description},
				{Key: "Status", Value: projectStatusLabel(project.Status)},
				{Key: "Public", Value: formatBool(project.IsPublic)},
			}

			if project.Parent != nil {
				pairs = append(pairs, output.KeyValue{
					Key:   "Parent",
					Value: fmt.Sprintf("%s (#%d)", project.Parent.Name, project.Parent.ID),
				})
			}

			pairs = append(pairs,
				output.KeyValue{Key: "Created", Value: project.CreatedOn},
				output.KeyValue{Key: "Updated", Value: project.UpdatedOn},
			)

			printer.Detail(pairs)
			return nil
		},
	}

	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}
