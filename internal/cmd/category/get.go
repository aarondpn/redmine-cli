package category

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/aarondpn/redmine-cli/v2/internal/cmdutil"
	"github.com/aarondpn/redmine-cli/v2/internal/ops"
	"github.com/aarondpn/redmine-cli/v2/internal/output"
)

func newCmdCategoryGet(f *cmdutil.Factory) *cobra.Command {
	var (
		format  string
		project string
	)

	cmd := &cobra.Command{
		Use:     "get <id-or-name>",
		Aliases: []string{"show", "view"},
		Short:   "Get issue category details",
		Long:    "Display an issue category: its project and default assignee. Accepts a numeric ID or category name (uses the default project, or pass --project).",
		Example: `  # By ID
  redmine categories get 12

  # By name within a project
  redmine categories get "Backend" --project myproject

  # JSON output
  redmine categories get 12 -o json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			client, err := f.ApiClient()
			if err != nil {
				return err
			}

			ctx := cmdutil.Context(cmd)
			id, err := resolveCategoryID(ctx, f, client, args[0], project)
			if err != nil {
				return err
			}

			printer := f.Printer(format)
			stop := printer.Spinner("Fetching category...")
			category, err := ops.GetIssueCategory(ctx, client, ops.GetIssueCategoryInput{ID: id})
			stop()
			if err != nil {
				return fmt.Errorf("failed to get category %d: %w", id, err)
			}

			if printer.Format() == output.FormatJSON {
				printer.JSON(category)
				return nil
			}

			assignee := ""
			if category.AssignedTo != nil {
				assignee = category.AssignedTo.Name
			}
			printer.Detail([]output.KeyValue{
				{Key: "ID", Value: output.StyleID.Render(fmt.Sprintf("%d", category.ID))},
				{Key: "Project", Value: category.Project.Name},
				{Key: "Name", Value: category.Name},
				{Key: "Assigned To", Value: assignee},
			})
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project name, identifier, or ID (for name resolution; falls back to default project)")
	cmdutil.AddOutputFlag(cmd, &format)

	_ = cmd.RegisterFlagCompletionFunc("project", cmdutil.CompleteProjects(f))

	return cmd
}
