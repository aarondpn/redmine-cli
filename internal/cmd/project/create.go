package project

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/models"
	"github.com/aarondpn/redmine-cli/internal/output"
)

func newCmdCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		name        string
		identifier  string
		description string
		public      bool
		parentID    int
		format      string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Create a new project",
		Long:    "Create a new Redmine project.",
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}
			printer := f.Printer(format)

			create := models.ProjectCreate{
				Name:       name,
				Identifier: identifier,
			}

			if description != "" {
				create.Description = description
			}
			if cmd.Flags().Changed("public") {
				create.IsPublic = &public
			}
			if parentID > 0 {
				create.ParentID = parentID
			}

			project, err := client.Projects.Create(context.Background(), create)
			if err != nil {
				return err
			}

			if format == output.FormatJSON {
				printer.JSON(project)
				return nil
			}

			printer.Success(fmt.Sprintf("Project %q created (ID: %d)", project.Name, project.ID))
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Project name (required)")
	cmd.Flags().StringVar(&identifier, "identifier", "", "Project identifier (required)")
	cmd.Flags().StringVar(&description, "description", "", "Project description")
	cmd.Flags().BoolVar(&public, "public", false, "Make project public")
	cmd.Flags().IntVar(&parentID, "parent", 0, "Parent project ID")

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("identifier")

	cmdutil.AddOutputFlag(cmd, &format)

	return cmd
}
