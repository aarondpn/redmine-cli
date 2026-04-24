package version

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
)

func newCmdVersionCreate(f *cmdutil.Factory) *cobra.Command {
	var (
		project       string
		name          string
		status        string
		sharing       string
		dueDate       string
		description   string
		wikiPageTitle string
		format        string
	)

	cmd := &cobra.Command{
		Use:     "create",
		Aliases: []string{"new"},
		Short:   "Create a project version",
		Long:    "Create a Redmine project version (milestone).",
		Example: `  # Create an open version in the default project
  redmine versions create --name 1.2.0

  # Create a version for a specific project
  redmine versions create --project myproject --name 1.2.0 --due-date 2026-06-30 --description "Release 1.2.0"`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			project, err = cmdutil.RequireProjectIdentifier(context.Background(), f, project)
			if err != nil {
				return err
			}

			create := models.VersionCreate{Name: name}
			if status != "" {
				create.Status = status
			}
			if sharing != "" {
				create.Sharing = sharing
			}
			if dueDate != "" {
				create.DueDate = cmdutil.ResolveDateKeyword(dueDate)
			}
			if description != "" {
				create.Description = description
			}
			if wikiPageTitle != "" {
				create.WikiPageTitle = wikiPageTitle
			}

			printer := f.Printer(format)
			version, err := client.Versions.Create(context.Background(), project, create)
			if err != nil {
				return err
			}

			printer.Resource(version, fmt.Sprintf("Version %q created (ID: %d)", version.Name, version.ID))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID (falls back to default project)")
	cmd.Flags().StringVar(&name, "name", "", "Version name (required)")
	cmd.Flags().StringVar(&status, "status", "", "Version status: open, locked, closed")
	cmd.Flags().StringVar(&sharing, "sharing", "", "Version sharing: none, descendants, hierarchy, tree, system")
	cmd.Flags().StringVar(&dueDate, "due-date", "", "Due date (YYYY-MM-DD or 'today')")
	cmd.Flags().StringVar(&description, "description", "", "Version description")
	cmd.Flags().StringVar(&wikiPageTitle, "wiki-page-title", "", "Wiki page title")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.MarkFlagRequired("name")
	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))
	_ = cmd.RegisterFlagCompletionFunc("status", cmdutil.CompleteVersionStatus)
	_ = cmd.RegisterFlagCompletionFunc("sharing", cmdutil.CompleteVersionSharing)

	return cmd
}
