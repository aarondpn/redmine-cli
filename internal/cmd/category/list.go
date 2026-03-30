package category

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/internal/output"
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

			project = cmdutil.DefaultProject(f, project)
			if project == "" {
				return fmt.Errorf("--project is required (or set a default project in config)")
			}

			project, err = cmdutil.ResolveProjectIdentifier(context.Background(), f, project)
			if err != nil {
				return err
			}

			printer := f.Printer(format)

			stop := printer.Spinner("Fetching categories...")
			categories, _, err := client.Categories.List(context.Background(), project)
			stop()
			if err != nil {
				return err
			}

			if cmdutil.HandleEmpty(printer, categories, "categories") {
				return nil
			}

			switch printer.Format() {
			case output.FormatJSON:
				printer.JSON(categories)
			case output.FormatCSV:
				headers := []string{"ID", "Name", "Assigned To"}
				rows := make([][]string, len(categories))
				for i, c := range categories {
					assignee := ""
					if c.AssignedTo != nil {
						assignee = c.AssignedTo.Name
					}
					rows[i] = []string{fmt.Sprintf("%d", c.ID), c.Name, assignee}
				}
				printer.CSV(headers, rows)
			default:
				headers := []string{"ID", "Name", "Assigned To"}
				rows := make([][]string, len(categories))
				for i, c := range categories {
					assignee := ""
					if c.AssignedTo != nil {
						assignee = c.AssignedTo.Name
					}
					rows[i] = []string{
						output.StyleID.Render(fmt.Sprintf("%d", c.ID)),
						c.Name,
						assignee,
					}
				}
				printer.Table(headers, rows)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
