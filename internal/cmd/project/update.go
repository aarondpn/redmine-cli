package project

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
)

func newCmdUpdate(f *cmdutil.Factory) *cobra.Command {
	var (
		name        string
		description string
		public      bool
	)

	cmd := &cobra.Command{
		Use:     "update <identifier>",
		Aliases: []string{"edit"},
		Short:   "Update a project",
		Long:    "Update an existing Redmine project.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer("")

			update := models.ProjectUpdate{}

			if cmd.Flags().Changed("name") {
				update.Name = &name
			}
			if cmd.Flags().Changed("description") {
				update.Description = &description
			}
			if cmd.Flags().Changed("public") {
				update.IsPublic = &public
			}

			err = client.Projects.Update(context.Background(), args[0], update)
			if err != nil {
				return err
			}

			printer.Success(fmt.Sprintf("Project %q updated", args[0]))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Project name")
	cmd.Flags().StringVar(&description, "description", "", "Project description")
	cmd.Flags().BoolVar(&public, "public", false, "Make project public")

	return cmd
}
