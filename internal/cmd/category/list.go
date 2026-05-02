package category

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/models"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

// NewCmdCategories creates the categories command group.
func NewCmdCategories(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "categories",
		Aliases: []string{"category"},
		Short:   "Manage issue categories",
	}

	cmd.AddCommand(newCmdCategoryList(f))
	return cmd
}

func newCmdCategoryList(f *cmdutil.Factory) *cobra.Command {
	var (
		project string
		format  string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List issue categories for a project",
		Aliases: []string{"ls"},
		Example: `  # List categories for a project
  redmine categories list --project myproject

  # JSON output
  redmine categories list --project myproject -o json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			project, err = cmdutil.RequireProjectIdentifier(context.Background(), f, project)
			if err != nil {
				return err
			}

			printer := f.Printer(format)

			stop := printer.Spinner("Fetching categories...")
			result, err := ops.ListCategories(context.Background(), client, ops.ListCategoriesInput{ProjectID: project})
			stop()
			if err != nil {
				return err
			}
			categories := result.Categories

			if cmdutil.HandleEmpty(printer, categories, "categories") {
				return nil
			}

			cmdutil.RenderCollection(printer, categories, []string{"ID", "Name", "Assigned To"}, func(c models.IssueCategory, styled bool) []string {
				id := fmt.Sprintf("%d", c.ID)
				assignee := ""
				if c.AssignedTo != nil {
					assignee = c.AssignedTo.Name
				}
				if styled {
					id = output.StyleID.Render(id)
				}
				return []string{id, c.Name, assignee}
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
