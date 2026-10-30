package category

import (
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
		Long: "List, inspect, create, update and delete issue categories.\n\n" +
			"A category groups the issues of one project and can carry a default assignee that is\n" +
			"preselected for new issues in it. Commands that take <id-or-name> accept a numeric ID or a\n" +
			"category name (resolved in --project or the default project).",
	}

	cmd.AddCommand(newCmdCategoryList(f))
	cmd.AddCommand(newCmdCategoryGet(f))
	cmd.AddCommand(newCmdCategoryCreate(f))
	cmd.AddCommand(newCmdCategoryUpdate(f))
	cmd.AddCommand(newCmdCategoryDelete(f))
	return cmd
}

func newCmdCategoryList(f *cmdutil.Factory) *cobra.Command {
	var (
		project string
		format  string
	)

	cmd := &cobra.Command{
		Use:     "list",
		Args:    cobra.NoArgs,
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

			project, err = cmdutil.RequireProjectIdentifier(cmdutil.Context(cmd), f, project)
			if err != nil {
				return err
			}

			printer := f.Printer(format)

			stop := printer.Spinner("Fetching categories...")
			result, err := ops.ListCategories(cmdutil.Context(cmd), client, ops.ListCategoriesInput{ProjectID: project})
			stop()
			if err != nil {
				return err
			}
			categories := result.Categories

			if cmdutil.HandleEmpty(printer, categories, "categories") {
				return nil
			}

			cmdutil.RenderCollection(printer, categories, []string{"ID", "Name", "Assigned To"}, func(c models.IssueCategory) []string {
				assignee := ""
				if c.AssignedTo != nil {
					assignee = c.AssignedTo.Name
				}
				return []string{output.StyleID.Render(fmt.Sprintf("%d", c.ID)), c.Name, assignee}
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
